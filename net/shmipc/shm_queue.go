package shmipc

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"unsafe"
)

var (
	ErrQueueFull  = errors.New("queue is full")
	errQueueEmpty = errors.New("queue is empty")
)

const (
	queueHeaderLength = 24   // Queue header size (see memory layout below)
	queueElementLen   = 12   // Size of each queue element (streamID + offset + status)
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
//   Offset 0-3:  streamID (uint32)       - Stream ID
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
	StreamID uint32 // Stream ID
	Offset   uint32 // Offset in shared memory buffer
	Status   uint32 // Stream status
}

// Queue represents a lock-free ring buffer queue for producer-consumer pattern
type Queue struct {
	mu          sync.Mutex
	head        *int64  // Consumer write, producer read
	tail        *int64  // Producer write, consumer read
	workingFlag *uint32 // 1 when peer is consuming, 0 otherwise
	cap         int64
	mem         []byte // Shared memory or process memory
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

func countQueueMemSize(queueCap int64) int {
	return queueHeaderLength + int(queueCap)*queueElementLen
}

func mappingQueue(mem []byte) *Queue {
	queueCap := int64(*(*uint32)(unsafe.Pointer(&mem[0])))

	q := &Queue{
		cap: queueCap,
	}

	queueStartOffset := queueHeaderLength
	queueEndOffset := queueHeaderLength + int(queueCap)*queueElementLen
	q.mem = mem[queueStartOffset:queueEndOffset]

	if IsARM {
		// ARM: cap(4) + workingFlag(4) + head(8) + tail(8)
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

func newQueue(mem []byte, queueCap int64) *Queue {
	if len(mem) < countQueueMemSize(queueCap) {
		panic("insufficient memory for queue")
	}

	*(*uint32)(unsafe.Pointer(&mem[0])) = uint32(queueCap)

	q := mappingQueue(mem)

	atomic.StoreInt64(q.head, 0)
	atomic.StoreInt64(q.tail, 0)
	atomic.StoreUint32(q.workingFlag, 0)

	return q
}

// Put adds an element to the queue (producer operation)
// Must lock to ensure atomicity of tail increment + element write
func (q *Queue) Put(e QueueElement) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	tail := atomic.LoadInt64(q.tail)

	if tail-atomic.LoadInt64(q.head) >= q.cap {
		return ErrQueueFull
	}

	queueOffset := int((tail % q.cap) * queueElementLen)

	// Write element struct directly (native endian)
	*(*QueueElement)(unsafe.Pointer(&q.mem[queueOffset])) = e

	atomic.AddInt64(q.tail, 1)

	return nil
}

// Pop removes an element from the queue (consumer operation)
// Lock-free read operation
func (q *Queue) Pop() (QueueElement, error) {
	head := atomic.LoadInt64(q.head)

	if head >= atomic.LoadInt64(q.tail) {
		return QueueElement{}, errQueueEmpty
	}

	queueOffset := int((head % q.cap) * queueElementLen)

	// Read element struct directly (native endian)
	e := *(*QueueElement)(unsafe.Pointer(&q.mem[queueOffset]))

	atomic.AddInt64(q.head, 1)

	return e, nil
}

// Size returns the number of elements in the queue
func (q *Queue) Size() int64 {
	return atomic.LoadInt64(q.tail) - atomic.LoadInt64(q.head)
}

// IsFull returns true if the queue is full
func (q *Queue) IsFull() bool {
	return q.Size() >= q.cap
}

// IsEmpty returns true if the queue is empty
func (q *Queue) IsEmpty() bool {
	return q.Size() <= 0
}

// MarkWorking sets the working flag
func (q *Queue) MarkWorking() bool {
	return atomic.CompareAndSwapUint32(q.workingFlag, 0, 1)
}

// MarkNotWorking clears the working flag, or sets to 1 if queue not empty
func (q *Queue) MarkNotWorking() {
	atomic.StoreUint32(q.workingFlag, 0)
}

// ConsumerIsWorking returns whether consumer is working
func (q *Queue) ConsumerIsWorking() bool {
	return atomic.LoadUint32(q.workingFlag) == 1
}

// newQueueManagerFromMem creates a new queue manager from provided memory
func newQueueManagerFromMem(path string, mem []byte, queueCap int64) (*QueueManager, error) {
	if queueCap <= 0 {
		queueCap = defaultQueueCap
	}

	singleQueueSize := countQueueMemSize(queueCap)
	totalSize := singleQueueSize * queueCount

	if len(mem) < totalSize {
		return nil, fmt.Errorf("insufficient memory: need %d bytes, have %d bytes", totalSize, len(mem))
	}

	qm := &QueueManager{
		path:        path,
		mem:         mem,
		mmapMapType: MemMapTypeDevShmFile,
		memFd:       -1,
	}

	// Create send and receive queues
	qm.sendQueue = newQueue(mem[:singleQueueSize], queueCap)
	qm.recvQueue = newQueue(mem[singleQueueSize:], queueCap)

	return qm, nil
}

// Cleanup releases queue manager resources
func (qm *QueueManager) Cleanup() error {
	qm.sendQueue = nil
	qm.recvQueue = nil
	return nil
}

// GetSendQueue returns the send queue
func (qm *QueueManager) GetSendQueue() *Queue {
	return qm.sendQueue
}

// GetRecvQueue returns the receive queue
func (qm *QueueManager) GetRecvQueue() *Queue {
	return qm.recvQueue
}

// GetPath returns the queue path or name
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
