package shmipc

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"unsafe"
)

var (
	ErrNoMoreBuffer  = errors.New("no more buffer available")
	ErrNotEnoughData = errors.New("not enough data")
)

// Shared Memory Buffer Layout (verified against legacy/buffer_manager.go)
//
// Structure: BufferManager -> BufferLists -> Buffers
//   [BufferManager Header (8B)]
//   [BufferList #1 Header (36B)] [Buffer Region: N×(Header(20B)+Data)]
//   [BufferList #2 Header (36B)] [Buffer Region: N×(Header(20B)+Data)]
//   ...
//
// BufferManager Header (8 bytes, native endian):
//   0-1:  listNum (uint16)     - Number of buffer lists
//   2-3:  reserved
//   4-7:  usedLength (uint32)  - Used memory excluding this header
//
// BufferList Header (36 bytes, native endian):
//   0-3:   size (int32)        - Free buffers count (atomic)
//   4-7:   cap (uint32)        - Max buffers in this list
//   8-11:  head (uint32)       - First free buffer offset (atomic, relative to buffer region)
//   12-15: tail (uint32)       - Last free buffer offset (atomic, relative to buffer region)
//   16-19: capPerBuffer (uint32) - Each buffer's data capacity
//   20-35: reserved
//
// Buffer Header (20 bytes, native endian):
//   0-3:   cap (uint32)        - Data capacity
//   4-7:   size (uint32)       - Current data size
//   8-11:  dataStart (uint32)  - Data start offset (for prepend)
//   12-15: nextBufferOffset (uint32) - Next buffer offset (for free list chain and buffer linking)
//   16:    flags (uint8)       - Bit 0: hasNext, Bit 1: inUsed
//   17-19: reserved
//   20+:   data bytes
//
// Design Notes:
// - Native endian (same machine IPC)
// - Lists sorted by capPerBuffer (ascending)
// - Free buffers form singly-linked list via nextBufferOffset
// - Lock-free Pop/Push using atomic CAS on head/tail
// - ARM: capPerBuffer must be multiple of 4

const (
	// Buffer header constants (legacy/buffer_manager.go:33-46)
	bufferHeaderSize      = 20 // cap(4) + size(4) + start(4) + next(4) + flags(4)
	bufferCapOffset       = 0
	bufferSizeOffset      = 4
	bufferDataStartOffset = 8
	nextBufferOffset      = 12
	bufferFlagOffset      = 16

	// Buffer list header size
	bufferListHeaderSize = 36 // size(4) + cap(4) + head(4) + tail(4) + capPerBuffer(4) + pushCount(4) + popCount(4) + reserved(8)

	// Buffer manager header size
	bufferManagerHeaderSize = 8 // listNum(2) + reserved(2) + usedLength(4)
	bmCapOffset             = 4
)

// Buffer flags (legacy/buffer_manager.go:48-51)
const (
	hasNextBufferFlag = 1 << iota // Buffer has next buffer in chain
	sliceInUsedFlag                // Buffer is currently in use
)

// BufferHeader represents the 20-byte buffer header in shared memory
type BufferHeader struct {
	cap              uint32 // Data capacity
	size             uint32 // Current data size
	dataStart        uint32 // Data start offset (for prepend)
	nextBufferOffset uint32 // Next buffer offset (for free list chain and buffer linking)
	flags            uint8  // Bit 0: hasNext, Bit 1: inUsed
	reserved         [3]byte
}

// headerFromBytes interprets a byte slice as a BufferHeader
func headerFromBytes(b []byte) *BufferHeader {
	return (*BufferHeader)(unsafe.Pointer(&b[0]))
}

// hasNext checks if buffer has a next buffer in chain
func (bh *BufferHeader) hasNext() bool {
	return (bh.flags & hasNextBufferFlag) > 0
}

// getNextBufferOffset returns the offset to the next buffer
func (bh *BufferHeader) getNextBufferOffset() uint32 {
	return bh.nextBufferOffset
}

// clearFlag clears all flags
func (bh *BufferHeader) clearFlag() {
	bh.flags = 0
}

// setInUsed marks buffer as in use
func (bh *BufferHeader) setInUsed() {
	bh.flags |= sliceInUsedFlag
}

// isInUsed checks if buffer is in use
func (bh *BufferHeader) isInUsed() bool {
	return (bh.flags & sliceInUsedFlag) > 0
}

// linkNext links this buffer to next buffer at given offset
func (bh *BufferHeader) linkNext(next uint32) {
	bh.nextBufferOffset = next
	bh.flags |= hasNextBufferFlag
}

// Buffer represents a single buffer with header and data
// Verified against legacy/buffer_slice.go:34-46
type Buffer struct {
	header    *BufferHeader // 20-byte header in shared memory
	data      []byte        // Data region in shared memory
	cap       uint32        // Buffer capacity
	start     uint32        // Data start offset (for prepend support)
	offset    uint32        // Offset in shared memory
	readIdx   int           // Read cursor
	writeIdx  int           // Write cursor
	isFromShm bool          // Whether from shared memory
	next      *Buffer
}

// newBuffer creates a new buffer
// Verified against legacy/buffer_slice.go:52-67
func newBuffer(header []byte, data []byte, offset uint32, isFromShm bool) *Buffer {
	bs := &Buffer{
		header:    headerFromBytes(header),
		data:      data,
		offset:    offset,
		isFromShm: isFromShm,
	}

	if isFromShm {
		bs.cap = bs.header.cap
		bs.start = bs.header.dataStart
		bs.readIdx = int(bs.start)
		bs.writeIdx = int(bs.start + bs.header.size)
	} else {
		bs.cap = uint32(cap(data))
	}

	return bs
}

// update writes buffer metadata back to header
// Verified against legacy/buffer_slice.go:107-115
func (bs *Buffer) update() {
	bs.header.size = uint32(bs.size())
	bs.header.dataStart = bs.start
	if bs.next != nil {
		bs.header.linkNext(bs.next.offset)
	}
}

// reset resets buffer to initial state
// Verified against legacy/buffer_slice.go:117-126
func (bs *Buffer) reset() {
	bs.header.size = 0
	bs.header.dataStart = 0
	bs.header.clearFlag()
	bs.writeIdx = 0
	bs.readIdx = 0
	bs.next = nil
}

// size returns the current data size
func (bs *Buffer) size() int {
	return bs.writeIdx - bs.readIdx
}

// remain returns remaining capacity
func (bs *Buffer) remain() int {
	return int(bs.cap) - bs.writeIdx
}

// Data returns the data slice for reading/writing
func (bs *Buffer) Data() []byte {
	return bs.data[bs.start:bs.start+bs.cap]
}

// Offset returns the offset in shared memory
func (bs *Buffer) Offset() uint32 {
	return bs.offset
}

// SetLen sets the length of valid data in the buffer
func (bs *Buffer) SetLen(length uint32) {
	bs.writeIdx = int(bs.start + length)
	// Update size in header
	bs.header.size = length
}

// BufferList represents a lock-free list of buffers of the same size
// Verified against legacy/buffer_manager.go:78-96
type BufferList struct {
	size         *int32  // Number of free buffers (atomic)
	cap          *uint32 // Max number of buffers
	head         *uint32 // Offset to first free buffer (atomic)
	tail         *uint32 // Offset to last free buffer (atomic)
	capPerBuf    *uint32 // Capacity of each buffer
	region       []byte  // Underlying memory for buffers
	regionOffset uint32  // Offset of buffer region in shared memory
	offset       uint32  // Offset of this list in shared memory
}

// Pop allocates a buffer from the free list (lock-free)
// Verified against legacy/buffer_manager.go:417-448
func (bl *BufferList) Pop() (*Buffer, error) {
	// Pre-decrement size counter to reserve a slot
	if atomic.AddInt32(bl.size, -1) <= 0 {
		atomic.AddInt32(bl.size, 1)
		return nil, ErrNoMoreBuffer
	}

	// Retry up to 200 times for lock-free CAS operations
	head := atomic.LoadUint32(bl.head)
	for i := 0; i < 200; i++ {
		bh := headerFromBytes(bl.region[head : head+bufferHeaderSize])

		// Try to move head to next buffer
		if bh.hasNext() && atomic.CompareAndSwapUint32(bl.head, head, bh.getNextBufferOffset()) {
			// Successfully claimed this buffer
			bh.clearFlag()
			bh.setInUsed()
			dataStart := head + bufferHeaderSize
			return newBuffer(bl.region[head:head+bufferHeaderSize], bl.region[dataStart:dataStart+*bl.capPerBuf],
				head+bl.regionOffset, true), nil
		}

		// CAS failed or no next buffer, reload head and retry
		head = atomic.LoadUint32(bl.head)
	}

	// Failed after 200 retries, restore size counter
	atomic.AddInt32(bl.size, 1)
	return nil, ErrNoMoreBuffer
}

// Push returns a buffer to the free list (lock-free)
// Verified against legacy/buffer_manager.go:450-462
func (bl *BufferList) Push(buf *Buffer) {
	buf.reset()
	for {
		oldTail := atomic.LoadUint32(bl.tail)
		newTail := buf.offset - bl.regionOffset
		if atomic.CompareAndSwapUint32(bl.tail, oldTail, newTail) {
			headerFromBytes(bl.region[oldTail : oldTail+bufferHeaderSize]).linkNext(newTail)
			atomic.AddInt32(bl.size, 1)
			return
		}
	}
}

// Remain returns the number of free buffers
// Verified against legacy/buffer_manager.go:464-467
func (bl *BufferList) Remain() int {
	// When size is 1, don't allow pop (for concurrent safety)
	return int(atomic.LoadInt32(bl.size) - 1)
}

// SizePercentPair describes a buffer list's specification
// Verified against legacy/buffer_manager.go:98-104
type SizePercentPair struct {
	Size    uint32 // Single buffer slice capacity
	Percent uint32 // Percent of total shared memory
}

type sizePercentPairs []*SizePercentPair

var _ sort.Interface = &sizePercentPairs{}

func (s sizePercentPairs) Len() int           { return len(s) }
func (s sizePercentPairs) Less(i, j int) bool { return s[i].Size < s[j].Size }
func (s sizePercentPairs) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// BufferManager manages multiple buffer lists with different buffer sizes
// Verified against legacy/buffer_manager.go:61-71
type BufferManager struct {
	lists        []*BufferList // Ordered by capPerBuffer (ascending)
	mem          []byte        // Shared memory
	minSliceSize uint32        // Minimum buffer size
	maxSliceSize uint32        // Maximum buffer size
	path         string        // Path or name for identification
	refCount     int32         // Reference count (atomic)
}

// GlobalBufferManager manages all buffer managers
// Verified against legacy/buffer_manager.go:73-76
type GlobalBufferManager struct {
	sync.Mutex
	bms map[string]*BufferManager
}

var (
	globalBufferManagers = &GlobalBufferManager{
		bms: make(map[string]*BufferManager, 8),
	}
)

// ValidateSizePercentPairs validates buffer size configurations for ARM
// Verified against legacy/config.go:115-117
func ValidateSizePercentPairs(pairs []*SizePercentPair) error {
	if !IsARM {
		return nil
	}

	for _, pair := range pairs {
		if pair.Size%4 != 0 {
			return fmt.Errorf("ARM: SizePercentPair.Size (%d) must be a multiple of 4", pair.Size)
		}
	}
	return nil
}

// countBufferListMemSize calculates memory size for a buffer list
// Verified against legacy/buffer_manager.go:337-339
func countBufferListMemSize(bufferNum, capPerBuffer uint32) uint32 {
	return bufferListHeaderSize + bufferNum*(capPerBuffer+bufferHeaderSize)
}

// createBufferList creates a new buffer list with initialized buffers
// Verified against legacy/buffer_manager.go:341-391
func createBufferList(bufNum, capPerBuf uint32, mem []byte) (*BufferList, error) {
	if bufNum == 0 || capPerBuf == 0 {
		return nil, fmt.Errorf("bufferNum:%d or capPerBuffer:%d cannot be 0", bufNum, capPerBuf)
	}

	needSize := countBufferListMemSize(bufNum, capPerBuf)
	if len(mem) < int(needSize) {
		return nil, fmt.Errorf("mem's size is at least:%d but:%d needSize:%d",
			needSize, len(mem), needSize)
	}

	regionStart := uint32(bufferListHeaderSize)
	regionEnd := needSize
	if regionEnd <= regionStart {
		return nil, fmt.Errorf("regionStart:%d regionEnd:%d slice bounds out of range", regionStart, regionEnd)
	}

	bl := &BufferList{
		size:         (*int32)(unsafe.Pointer(&mem[0])),
		cap:          (*uint32)(unsafe.Pointer(&mem[4])),
		head:         (*uint32)(unsafe.Pointer(&mem[8])),
		tail:         (*uint32)(unsafe.Pointer(&mem[12])),
		capPerBuf:    (*uint32)(unsafe.Pointer(&mem[16])),
		region:       mem[regionStart:regionEnd],
		regionOffset: regionStart,
		offset:       0,
	}

	// Initialize header fields
	*bl.size = int32(bufNum)
	*bl.cap = bufNum
	*bl.head = 0
	*bl.tail = (bufNum - 1) * (capPerBuf + bufferHeaderSize)
	*bl.capPerBuf = capPerBuf

	// Initialize buffer chain
	cur, next := uint32(0), uint32(0)
	for i := 0; i < int(bufNum); i++ {
		next = cur + capPerBuf + bufferHeaderSize

		// Set buffer's header (native endian)
		*(*uint32)(unsafe.Pointer(&bl.region[cur])) = capPerBuf
		*(*uint32)(unsafe.Pointer(&bl.region[cur+bufferSizeOffset])) = 0
		*(*uint32)(unsafe.Pointer(&bl.region[cur+bufferDataStartOffset])) = 0

		if i < int(bufNum-1) {
			*(*uint32)(unsafe.Pointer(&bl.region[cur+nextBufferOffset])) = next
			bl.region[cur+bufferFlagOffset] |= hasNextBufferFlag
		}
		cur = next
	}

	// Clear flag on tail buffer
	headerFromBytes(bl.region[*bl.tail:]).clearFlag()

	return bl, nil
}

// mappingBufferList maps an existing buffer list from shared memory
// Verified against legacy/buffer_manager.go:393-415
func mappingBufferList(mem []byte) (*BufferList, error) {
	if len(mem) < bufferListHeaderSize {
		return nil, fmt.Errorf("mappingBufferList failed, mem's size is at least %d", bufferListHeaderSize)
	}

	bl := &BufferList{
		size:      (*int32)(unsafe.Pointer(&mem[0])),
		cap:       (*uint32)(unsafe.Pointer(&mem[4])),
		head:      (*uint32)(unsafe.Pointer(&mem[8])),
		tail:      (*uint32)(unsafe.Pointer(&mem[12])),
		capPerBuf: (*uint32)(unsafe.Pointer(&mem[16])),
		offset:    0,
	}

	needSize := countBufferListMemSize(*bl.cap, *bl.capPerBuf)
	if needSize > uint32(len(mem)) || needSize < bufferListHeaderSize {
		return nil, fmt.Errorf("mappingBufferList failed, size:%d cap:%d head:%d tail:%d capPerBuf:%d err: mem's size is at least %d but:%d",
			*bl.size, *bl.cap, *bl.head, *bl.tail, *bl.capPerBuf, needSize, len(mem))
	}

	bl.region = mem[bufferListHeaderSize:needSize]
	bl.regionOffset = bufferListHeaderSize

	return bl, nil
}

// CreateBufferManager creates a new buffer manager
// Verified against legacy/buffer_manager.go:259-297
func CreateBufferManager(listSizePercent []*SizePercentPair, path string, mem []byte) (*BufferManager, error) {
	if len(mem) == 0 {
		return nil, fmt.Errorf("mem cannot be empty")
	}

	// Calculate buffer region capacity
	bufferRegionCap := uint64(len(mem) - bufferListHeaderSize*len(listSizePercent) - bufferManagerHeaderSize)

	// Write number of lists (native endian)
	*(*uint16)(unsafe.Pointer(&mem[0])) = uint16(len(listSizePercent))

	hadUsedOffset := uint32(bufferManagerHeaderSize)
	freeBufferLists := make([]*BufferList, 0, len(listSizePercent))
	sumPercent := uint32(0)

	for _, pair := range listSizePercent {
		sumPercent += pair.Percent
		if sumPercent > 100 {
			return nil, errors.New("the sum of all SizePercentPair's percent must be equals 100")
		}

		bufferNum := uint32(bufferRegionCap*uint64(pair.Percent)/100) / (pair.Size + bufferHeaderSize)
		needSize := countBufferListMemSize(bufferNum, pair.Size)

		freeList, err := createBufferList(bufferNum, pair.Size, mem[hadUsedOffset:])
		if err != nil {
			return nil, err
		}

		freeBufferLists = append(freeBufferLists, freeList)
		hadUsedOffset += needSize
	}

	ret := &BufferManager{
		path:         path,
		mem:          mem,
		lists:        freeBufferLists,
		minSliceSize: listSizePercent[0].Size,
		maxSliceSize: listSizePercent[len(listSizePercent)-1].Size,
		refCount:     1,
	}

	// Write used length (native endian)
	*(*uint32)(unsafe.Pointer(&mem[bmCapOffset])) = hadUsedOffset - uint32(bufferManagerHeaderSize)

	return ret, nil
}

// MappingBufferManager maps an existing buffer manager from shared memory
// Verified against legacy/buffer_manager.go:299-335
func MappingBufferManager(path string, mem []byte) (*BufferManager, error) {
	if len(mem) <= bmCapOffset {
		return nil, fmt.Errorf("mem's size is at least:%d but:%d", bmCapOffset+1, len(mem))
	}

	listNum := int(*(*uint16)(unsafe.Pointer(&mem[0])))
	freeLists := make([]*BufferList, 0, listNum)
	length := *(*uint32)(unsafe.Pointer(&mem[bmCapOffset]))

	if len(mem) < bufferManagerHeaderSize+int(length) || listNum == 0 {
		return nil, fmt.Errorf("could not mappingBufferManager, listNum:%d len(mem) at least:%d but:%d",
			listNum, length+bufferManagerHeaderSize, len(mem))
	}

	hadUsedOffset := uint32(bufferManagerHeaderSize)

	for i := 0; i < listNum; i++ {
		l, err := mappingBufferList(mem[hadUsedOffset:])
		if err != nil {
			return nil, err
		}

		size := countBufferListMemSize(*l.cap, *l.capPerBuf)
		hadUsedOffset += size
		freeLists = append(freeLists, l)
	}

	ret := &BufferManager{
		path:         path,
		mem:          mem,
		minSliceSize: *freeLists[0].capPerBuf,
		maxSliceSize: *freeLists[len(freeLists)-1].capPerBuf,
		lists:        freeLists,
		refCount:     1,
	}

	return ret, nil
}


// AllocBuffer allocates a single buffer of given size
// Verified against legacy/buffer_manager.go:482-495
func (b *BufferManager) AllocBuffer(size uint32) (*Buffer, error) {
	if size <= b.maxSliceSize {
		for i := range b.lists {
			if size <= *b.lists[i].capPerBuf {
				buf, err := b.lists[i].Pop()
				if err != nil {
					continue
				}
				return buf, nil
			}
		}
	}
	return nil, ErrNoMoreBuffer
}

// RecycleBuffer returns a buffer to the appropriate free list
// Verified against legacy/buffer_manager.go:514-528
func (b *BufferManager) RecycleBuffer(slice *Buffer) {
	if slice == nil {
		return
	}
	if slice.isFromShm {
		for i := range b.lists {
			if slice.cap == *b.lists[i].capPerBuf {
				b.lists[i].Push(slice)
				break
			}
		}
	}
}

// ReadBuffer reads a buffer from shared memory at given offset
// Verified against legacy/buffer_manager.go:559-571
func (b *BufferManager) ReadBuffer(offset uint32) (*Buffer, error) {
	if int(offset)+bufferHeaderSize >= len(b.mem) {
		return nil, fmt.Errorf("broken share memory. readBufferSlice unexpected offset:%d buffers cap:%d",
			offset, len(b.mem))
	}

	bufCap := *(*uint32)(unsafe.Pointer(&b.mem[offset+bufferCapOffset]))
	bufEndOffset := offset + uint32(bufferHeaderSize) + bufCap

	if bufEndOffset > uint32(len(b.mem)) {
		return nil, fmt.Errorf("broken share memory. readBuffer unexpected bufferEndOffset:%d. bufferStartOffset:%d buffers cap:%d",
			bufEndOffset, offset, len(b.mem))
	}

	return newBuffer(b.mem[offset:offset+bufferHeaderSize], b.mem[offset+bufferHeaderSize:bufEndOffset], offset, true), nil
}


// GetPath returns the buffer manager path
func (b *BufferManager) GetPath() string {
	return b.path
}
