package shmipc

import (
	"errors"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

var (
	ErrQueueFull  = errors.New("queue is full")
	errQueueEmpty = errors.New("queue is empty")
)

const (
	queueHeaderLength = 24   // Queue header size (see memory layout below)
	queueElementLen   = 12   // Size of each queue element (seqID + offset + status)
	queueCount        = 2    // Always 2 queues (send + recv)
	defaultQueueCap   = 8192 // Default capacity: 8192 elements (legacy default is 16384)
)

// Queue Memory Layout (verified against legacy/queue.go):
//
// ARM Architecture (8-byte alignment required for int64 pointers):
//   Offset 0-3:   capacity (uint32, native endian)
//   Offset 4-7:   workingFlag (uint32)
//   Offset 8-15:  head pointer (int64, 8-byte aligned)
//   Offset 16-23: tail pointer (int64, 8-byte aligned)
//   Offset 24+:   queue elements array
//
// x86 Architecture (no strict alignment requirement, but layout has design issue):
//   Offset 0-3:   capacity (uint32, native endian)
//   Offset 4-11:  head pointer (int64) - NOTE: starts at offset 4, not 8-byte aligned
//   Offset 12-19: tail pointer (int64) - NOTE: starts at offset 12, not 8-byte aligned
//   Offset 20-23: workingFlag (uint32)
//   Offset 24+:   queue elements array
//   DESIGN ISSUE: The int64 pointers are not 8-byte aligned on x86 (head at offset 4, tail at 12).
//                 While x86 tolerates misaligned access, it causes performance degradation.
//                 This layout is kept for compatibility with legacy implementation.
//
// Queue Element Layout (12 bytes, native endian):
//   Offset 0-3:  seqID (uint32)          - Stream ID
//   Offset 4-7:  offsetInShmBuf (uint32) - Offset in shared memory buffer
//   Offset 8-11: status (uint32)         - Stream status
//
// Notes:
// - All integers use NATIVE ENDIAN (direct pointer casting)
// - Shared memory is always between processes on the same machine
// - ARM requires int64 pointers to be 8-byte aligned
// - x86 has no alignment requirements but uses different layout

// QueueElement represents metadata for data in shared memory buffer
type QueueElement struct {
	SeqID          uint32 // Stream ID
	OffsetInShmBuf uint32 // Offset in shared memory buffer
	Status         uint32 // Stream status
}

// Queue represents a lock-free ring buffer queue for producer-consumer pattern
type Queue struct {
	mu                 sync.Mutex
	head               *int64  // Consumer write, producer read
	tail               *int64  // Producer write, consumer read
	workingFlag        *uint32 // 1 when peer is consuming, 0 otherwise
	cap                int64
	queueBytesOnMemory []byte // Shared memory or process memory
}

// QueueManager manages send and receive queues in shared memory
type QueueManager struct {
	path        string
	sendQueue   *Queue
	recvQueue   *Queue
	mem         []byte
	mmapMapType MemMapType
	memFd       int
}

// countQueueMemSize calculates memory size needed for a single queue
// Note: Legacy implementation does NOT include alignment padding here.
// The alignment is already ensured by queueHeaderLength being 24 bytes.
func countQueueMemSize(queueCap int64) int {
	return queueHeaderLength + int(queueCap)*queueElementLen
}

// mappingQueue maps an existing queue from shared memory bytes
// This matches legacy mappingQueueFromBytes (legacy/queue.go:188-209)
func mappingQueue(mem []byte) *Queue {
	// Read capacity from memory (native endian)
	queueCap := int64(*(*uint32)(unsafe.Pointer(&mem[0])))

	q := &Queue{
		cap: queueCap,
	}

	// Map queue header fields based on architecture
	// queueBytesOnMemory points to the element array AFTER the header
	queueStartOffset := queueHeaderLength
	queueEndOffset := queueHeaderLength + int(queueCap)*queueElementLen
	q.queueBytesOnMemory = mem[queueStartOffset:queueEndOffset]

	if IsARM {
		// ARM requires 8-byte alignment for int64
		// Layout: cap(4) + workingFlag(4) + head(8) + tail(8)
		q.workingFlag = (*uint32)(unsafe.Pointer(&mem[4]))
		q.head = (*int64)(unsafe.Pointer(&mem[8]))
		q.tail = (*int64)(unsafe.Pointer(&mem[16]))
	} else {
		// x86: cap(4) + head(8) + tail(8) + workingFlag(4)
		q.head = (*int64)(unsafe.Pointer(&mem[4]))
		q.tail = (*int64)(unsafe.Pointer(&mem[12]))
		q.workingFlag = (*uint32)(unsafe.Pointer(&mem[20]))
	}

	return q
}

// newQueue creates a new queue from shared memory bytes and initializes it
// This matches legacy createQueueFromBytes (legacy/queue.go:179-186)
func newQueue(mem []byte, queueCap int64) *Queue {
	if len(mem) < countQueueMemSize(queueCap) {
		panic("insufficient memory for queue")
	}

	// Write capacity using native endian (direct pointer cast)
	*(*uint32)(unsafe.Pointer(&mem[0])) = uint32(queueCap)

	// Map the queue structure
	q := mappingQueue(mem)

	// Initialize head, tail, workingFlag to zero
	atomic.StoreInt64(q.head, 0)
	atomic.StoreInt64(q.tail, 0)
	atomic.StoreUint32(q.workingFlag, 0)

	return q
}

// Put adds an element to the queue (producer operation)
// Must lock to ensure atomicity of tail increment + element write
// (verified against legacy/queue.go:262-279)
func (q *Queue) Put(e QueueElement) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	tail := atomic.LoadInt64(q.tail)

	// Check if queue is full
	if tail-atomic.LoadInt64(q.head) >= q.cap {
		return ErrQueueFull
	}

	// Calculate position in ring buffer (relative to queueBytesOnMemory start)
	// Note: queueBytesOnMemory already points past the header (offset 24)
	queueOffset := int((tail % q.cap) * queueElementLen)

	// Write element fields (native endian, direct pointer casting)
	*(*uint32)(unsafe.Pointer(&q.queueBytesOnMemory[queueOffset])) = e.SeqID
	*(*uint32)(unsafe.Pointer(&q.queueBytesOnMemory[queueOffset+4])) = e.OffsetInShmBuf
	*(*uint32)(unsafe.Pointer(&q.queueBytesOnMemory[queueOffset+8])) = e.Status

	// Increment tail atomically
	atomic.AddInt64(q.tail, 1)

	return nil
}

// Pop removes an element from the queue (consumer operation)
// Lock-free read operation (verified against legacy/queue.go:247-260)
func (q *Queue) Pop() (QueueElement, error) {
	// Atomic ensures the data that peer writes to shared memory can be seen
	head := atomic.LoadInt64(q.head)

	// Check if queue is empty
	if head >= atomic.LoadInt64(q.tail) {
		return QueueElement{}, errQueueEmpty
	}

	// Calculate position in ring buffer (relative to queueBytesOnMemory start)
	// Note: queueBytesOnMemory already points past the header (offset 24)
	queueOffset := int((head % q.cap) * queueElementLen)

	// Read element fields (native endian, direct pointer casting)
	e := QueueElement{
		SeqID:          *(*uint32)(unsafe.Pointer(&q.queueBytesOnMemory[queueOffset])),
		OffsetInShmBuf: *(*uint32)(unsafe.Pointer(&q.queueBytesOnMemory[queueOffset+4])),
		Status:         *(*uint32)(unsafe.Pointer(&q.queueBytesOnMemory[queueOffset+8])),
	}

	// Increment head atomically
	atomic.AddInt64(q.head, 1)

	return e, nil
}

// Size returns the current number of elements in the queue
func (q *Queue) Size() int64 {
	return atomic.LoadInt64(q.tail) - atomic.LoadInt64(q.head)
}

// IsFull returns whether the queue is full
func (q *Queue) IsFull() bool {
	return q.Size() >= q.cap
}

// IsEmpty returns whether the queue is empty
func (q *Queue) IsEmpty() bool {
	return q.Size() <= 0
}

// MarkWorking sets the working flag to indicate consumer is processing
func (q *Queue) MarkWorking() bool {
	return atomic.CompareAndSwapUint32(q.workingFlag, 0, 1)
}

// MarkNotWorking clears the working flag
// If queue is not empty, sets it back to 1 to prevent excessive polling
// (verified against legacy/queue.go:289-296)
func (q *Queue) MarkNotWorking() {
	atomic.StoreUint32(q.workingFlag, 0)
	if !q.IsEmpty() {
		atomic.StoreUint32(q.workingFlag, 1)
	}
}

// ConsumerIsWorking returns whether the consumer is actively processing
func (q *Queue) ConsumerIsWorking() bool {
	return atomic.LoadUint32(q.workingFlag) == 1
}

// CreateQueueManager creates a new queue manager with file-based shared memory
func CreateQueueManager(shmPath string, queueCap int64) (*QueueManager, error) {
	if queueCap <= 0 {
		queueCap = defaultQueueCap
	}

	singleQueueSize := countQueueMemSize(queueCap)
	totalSize := singleQueueSize * queueCount

	// Create shared memory file
	file, err := syscall.Open(shmPath, syscall.O_RDWR|syscall.O_CREAT|syscall.O_TRUNC, 0600)
	if err != nil {
		return nil, err
	}
	defer syscall.Close(file)

	// Set file size
	if err := syscall.Ftruncate(file, int64(totalSize)); err != nil {
		return nil, err
	}

	// Memory map the file
	mem, err := unix.Mmap(file, 0, totalSize, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		return nil, err
	}

	qm := &QueueManager{
		path:        shmPath,
		mem:         mem,
		mmapMapType: MemMapTypeDevShmFile,
		memFd:       -1,
	}

	// Create send and receive queues
	qm.sendQueue = newQueue(mem[:singleQueueSize], queueCap)
	qm.recvQueue = newQueue(mem[singleQueueSize:], queueCap)

	return qm, nil
}

// CreateQueueManagerWithMemFd creates a new queue manager with memfd-based shared memory
func CreateQueueManagerWithMemFd(queuePathName string, queueCap int64) (*QueueManager, error) {
	if queueCap <= 0 {
		queueCap = defaultQueueCap
	}

	singleQueueSize := countQueueMemSize(queueCap)
	totalSize := singleQueueSize * queueCount

	// Create memfd
	fd, err := unix.MemfdCreate(queuePathName, unix.MFD_CLOEXEC)
	if err != nil {
		return nil, err
	}

	// Set size
	if err := unix.Ftruncate(fd, int64(totalSize)); err != nil {
		syscall.Close(fd)
		return nil, err
	}

	// Memory map the memfd
	mem, err := unix.Mmap(fd, 0, totalSize, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		syscall.Close(fd)
		return nil, err
	}

	qm := &QueueManager{
		path:        queuePathName,
		mem:         mem,
		mmapMapType: MemMapTypeMemFd,
		memFd:       fd,
	}

	// Create send and receive queues
	qm.sendQueue = newQueue(mem[:singleQueueSize], queueCap)
	qm.recvQueue = newQueue(mem[singleQueueSize:], queueCap)

	return qm, nil
}

// MappingQueueManager maps existing file-based shared memory
func MappingQueueManager(shmPath string) (*QueueManager, error) {
	// Open existing shared memory file
	file, err := syscall.Open(shmPath, syscall.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	defer syscall.Close(file)

	// Get file size
	var stat syscall.Stat_t
	if err := syscall.Fstat(file, &stat); err != nil {
		return nil, err
	}

	totalSize := int(stat.Size)
	singleQueueSize := totalSize / queueCount

	// Memory map the file
	mem, err := unix.Mmap(file, 0, totalSize, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		return nil, err
	}

	qm := &QueueManager{
		path:        shmPath,
		mem:         mem,
		mmapMapType: MemMapTypeDevShmFile,
		memFd:       -1,
	}

	// Map queues (reversed: client's send is server's recv)
	// Use mappingQueue to read existing queue without reinitializing
	qm.recvQueue = mappingQueue(mem[:singleQueueSize])
	qm.sendQueue = mappingQueue(mem[singleQueueSize:])

	return qm, nil
}

// MappingQueueManagerMemfd maps existing memfd-based shared memory
func MappingQueueManagerMemfd(queuePathName string, memFd int) (*QueueManager, error) {
	// Get memfd size
	var stat syscall.Stat_t
	if err := syscall.Fstat(memFd, &stat); err != nil {
		return nil, err
	}

	totalSize := int(stat.Size)
	singleQueueSize := totalSize / queueCount

	// Memory map the memfd
	mem, err := unix.Mmap(memFd, 0, totalSize, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		return nil, err
	}

	qm := &QueueManager{
		path:        queuePathName,
		mem:         mem,
		mmapMapType: MemMapTypeMemFd,
		memFd:       memFd,
	}

	// Map queues (reversed: client's send is server's recv)
	// Use mappingQueue to read existing queue without reinitializing
	qm.recvQueue = mappingQueue(mem[:singleQueueSize])
	qm.sendQueue = mappingQueue(mem[singleQueueSize:])

	return qm, nil
}

// Cleanup releases queue manager resources
func (qm *QueueManager) Cleanup() error {
	var lastErr error

	// Unmap memory
	if qm.mem != nil {
		if err := unix.Munmap(qm.mem); err != nil {
			lastErr = err
		}
		qm.mem = nil
	}

	// Close memfd if applicable
	if qm.memFd >= 0 {
		if err := syscall.Close(qm.memFd); err != nil {
			lastErr = err
		}
		qm.memFd = -1
	}

	// Remove file if file-based
	if qm.mmapMapType == MemMapTypeDevShmFile && qm.path != "" {
		if err := syscall.Unlink(qm.path); err != nil && err != syscall.ENOENT {
			lastErr = err
		}
	}

	return lastErr
}

// GetSendQueue returns the send queue
func (qm *QueueManager) GetSendQueue() *Queue {
	return qm.sendQueue
}

// GetRecvQueue returns the receive queue
func (qm *QueueManager) GetRecvQueue() *Queue {
	return qm.recvQueue
}

// GetPath returns the queue path
func (qm *QueueManager) GetPath() string {
	return qm.path
}

// GetMemFd returns the memfd file descriptor
func (qm *QueueManager) GetMemFd() int {
	return qm.memFd
}

// GetType returns the memory mapping type
func (qm *QueueManager) GetType() MemMapType {
	return qm.mmapMapType
}
