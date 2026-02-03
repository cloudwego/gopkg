package malloc

import (
	"fmt"
	"math/bits"
	"unsafe"
)

const (
	// bitmapHeaderSize is the header added to each allocation.
	bitmapHeaderSize = 8

	// bitmapMagic detects double-free/invalid blocks.
	bitmapMagic uint32 = 0xB17BA900

	// DefaultBitmapMinBlockSize is the default minimum block size (4KB).
	DefaultBitmapMinBlockSize = 4 * 1024

	// DefaultBitmapMaxBlockSize is the default maximum block size (512KB).
	DefaultBitmapMaxBlockSize = 512 * 1024
)

// BitmapAllocator manages memory using a bitmap to track block usage.
// Each bit represents one block of minBlockSize bytes.
// Supports variable-size allocations using contiguous blocks.
type BitmapAllocator struct {
	arena      []byte
	arenaStart unsafe.Pointer

	// bitmap is stored at the start of arena
	bitmap []byte

	// blocks points to the usable memory after bitmap
	blocksStart unsafe.Pointer
	numBlocks   int

	// next-fit: start searching from here
	nextIdx int

	minBlockSize  int
	minBlockShift int // log2(minBlockSize) for fast division
	maxBlockSize  int
}

// NewBitmapAllocator creates a bitmap allocator with default block sizes (4KB min, 512KB max).
// Arena size must be large enough to hold bitmap + at least one maxBlock allocation.
func NewBitmapAllocator(arena []byte) (*BitmapAllocator, error) {
	return NewBitmapAllocatorWithBlockSize(arena, DefaultBitmapMinBlockSize, DefaultBitmapMaxBlockSize)
}

// NewBitmapAllocatorWithBlockSize creates a bitmap allocator with specified block sizes.
// minBlock must be >= 4096 and a multiple of 4096.
// maxBlock must be > minBlock and a multiple of minBlock.
// Arena size must be large enough to hold bitmap + at least one maxBlock allocation.
func NewBitmapAllocatorWithBlockSize(arena []byte, minBlock, maxBlock int) (*BitmapAllocator, error) {
	if minBlock < 4096 {
		return nil, fmt.Errorf("minBlockSize must be >= 4096, got %d", minBlock)
	}
	if minBlock%4096 != 0 {
		return nil, fmt.Errorf("minBlockSize must be multiple of 4096, got %d", minBlock)
	}
	if maxBlock <= minBlock {
		return nil, fmt.Errorf("maxBlockSize (%d) must be > minBlockSize (%d)", maxBlock, minBlock)
	}
	if maxBlock%minBlock != 0 {
		return nil, fmt.Errorf("maxBlockSize must be multiple of minBlockSize, got %d %% %d = %d",
			maxBlock, minBlock, maxBlock%minBlock)
	}

	arenaSize := len(arena)

	// Calculate bitmap size: each bit covers minBlock bytes
	// bitmap is aligned to minBlock so blocks start aligned
	// Solve: bitmapBlocks * minBlock + numBlocks * minBlock = arenaSize
	//        bitmapBlocks * minBlock * 8 >= numBlocks (each byte covers 8 blocks)
	// Approximate: bitmapBytes = ceil(arenaSize / (8 * minBlock + 1))
	bitmapBytes := (arenaSize + 8*minBlock) / (8*minBlock + 1)
	// Round up to minBlock alignment
	bitmapSize := ((bitmapBytes + minBlock - 1) / minBlock) * minBlock

	blocksSize := arenaSize - bitmapSize
	numBlocks := blocksSize / minBlock

	if numBlocks < maxBlock/minBlock {
		return nil, fmt.Errorf("arena too small: need at least %d blocks, got %d",
			maxBlock/minBlock, numBlocks)
	}

	a := &BitmapAllocator{
		arena:         arena,
		arenaStart:    unsafe.Pointer(&arena[0]),
		bitmap:        arena[:bitmapBytes],
		blocksStart:   unsafe.Pointer(&arena[bitmapSize]),
		numBlocks:     numBlocks,
		minBlockSize:  minBlock,
		minBlockShift: bits.TrailingZeros(uint(minBlock)),
		maxBlockSize:  maxBlock,
	}

	// Clear bitmap (all free)
	for i := range a.bitmap {
		a.bitmap[i] = 0
	}

	return a, nil
}

// Alloc allocates memory of at least size bytes.
// Returns nil if no contiguous region is available.
func (a *BitmapAllocator) Alloc(size int) []byte {
	if size <= 0 {
		return nil
	}

	totalSize := size + bitmapHeaderSize
	if totalSize > a.maxBlockSize {
		return nil
	}

	// Calculate blocks needed using bit shift
	blocksNeeded := (totalSize + a.minBlockSize - 1) >> a.minBlockShift

	// Fast path: single block allocation
	if blocksNeeded == 1 {
		idx := a.findFreeBit(a.nextIdx)
		if idx == -1 && a.nextIdx > 0 {
			idx = a.findFreeBit(0)
		}
		if idx == -1 {
			return nil
		}
		// Set bit
		a.bitmap[idx>>3] |= 1 << (idx & 7)
		a.nextIdx = idx + 1
		if a.nextIdx >= a.numBlocks {
			a.nextIdx = 0
		}
		ptr := unsafe.Add(a.blocksStart, idx<<a.minBlockShift)
		*(*uint32)(ptr) = bitmapMagic
		*(*uint32)(unsafe.Add(ptr, 4)) = uint32(size)
		return unsafe.Slice((*byte)(unsafe.Add(ptr, bitmapHeaderSize)), a.minBlockSize-bitmapHeaderSize)[:size]
	}

	// Multi-block: next-fit search with wrap around
	startIdx := a.findFreeRun(a.nextIdx, blocksNeeded)
	if startIdx == -1 && a.nextIdx > 0 {
		startIdx = a.findFreeRun(0, blocksNeeded)
	}
	if startIdx == -1 {
		return nil
	}

	// Mark blocks as used
	a.setBlocks(startIdx, blocksNeeded, true)
	a.nextIdx = startIdx + blocksNeeded
	if a.nextIdx >= a.numBlocks {
		a.nextIdx = 0
	}

	// Write header
	ptr := unsafe.Add(a.blocksStart, startIdx<<a.minBlockShift)
	*(*uint32)(ptr) = bitmapMagic
	*(*uint32)(unsafe.Add(ptr, 4)) = uint32(size)

	blockBytes := blocksNeeded << a.minBlockShift
	return unsafe.Slice((*byte)(unsafe.Add(ptr, bitmapHeaderSize)), blockBytes-bitmapHeaderSize)[:size]
}

// Free returns memory to the allocator.
// Panics if block is invalid or already freed.
func (a *BitmapAllocator) Free(block []byte) {
	if cap(block) == 0 {
		return
	}

	// Get original allocation pointer
	dataPtr := *(*uintptr)(unsafe.Pointer(&block))
	headerPtr := unsafe.Pointer(dataPtr - bitmapHeaderSize)

	// Validate pointer is in blocks region
	offset := int(uintptr(headerPtr) - uintptr(a.blocksStart))
	if offset < 0 || offset >= a.numBlocks<<a.minBlockShift {
		panic("bitmap: block not in arena")
	}

	// Check alignment
	if offset&(a.minBlockSize-1) != 0 {
		panic("bitmap: misaligned block")
	}

	// Verify magic
	if *(*uint32)(headerPtr) != bitmapMagic {
		panic("bitmap: double free or invalid block")
	}

	// Get stored size and calculate blocks
	storedSize := int(*(*uint32)(unsafe.Add(headerPtr, 4)))
	if storedSize > cap(block) {
		panic("bitmap: corrupted size")
	}

	totalSize := storedSize + bitmapHeaderSize
	blocksUsed := (totalSize + a.minBlockSize - 1) >> a.minBlockShift
	startIdx := offset >> a.minBlockShift

	// Clear magic
	*(*uint32)(headerPtr) = 0

	// Fast path: single block
	if blocksUsed == 1 {
		a.bitmap[startIdx>>3] &^= 1 << (startIdx & 7)
		return
	}

	// Mark blocks as free
	a.setBlocks(startIdx, blocksUsed, false)
}

// Available returns total free bytes.
func (a *BitmapAllocator) Available() int {
	free := 0
	for i := 0; i < a.numBlocks; i++ {
		if !a.isSet(i) {
			free += a.minBlockSize - bitmapHeaderSize
		}
	}
	return free
}

// Reset clears all allocations and returns the allocator to its initial state.
func (a *BitmapAllocator) Reset() {
	for i := range a.bitmap {
		a.bitmap[i] = 0
	}
	a.nextIdx = 0
}

// IsValidOffset checks if the given data offset could be a valid allocation start.
// The offset is relative to blocksStart (not arena start).
// Validates bounds and alignment without checking magic (allocation state).
func (a *BitmapAllocator) IsValidOffset(dataOffset int) bool {
	blockOffset := dataOffset - bitmapHeaderSize
	if blockOffset < 0 || blockOffset >= a.numBlocks<<a.minBlockShift {
		return false
	}
	return blockOffset&(a.minBlockSize-1) == 0
}

// FreeAt returns a block at the given data offset to the allocator.
// The offset is relative to blocksStart and should point to user data.
// Panics if the offset is invalid or the block doesn't belong to this allocator.
func (a *BitmapAllocator) FreeAt(dataOffset int) {
	blockOffset := dataOffset - bitmapHeaderSize
	if blockOffset < 0 || blockOffset >= a.numBlocks<<a.minBlockShift {
		panic("bitmap: offset out of range")
	}
	if blockOffset&(a.minBlockSize-1) != 0 {
		panic("bitmap: misaligned offset")
	}

	headerPtr := unsafe.Add(a.blocksStart, blockOffset)
	if *(*uint32)(headerPtr) != bitmapMagic {
		panic("bitmap: double free or invalid block")
	}

	storedSize := int(*(*uint32)(unsafe.Add(headerPtr, 4)))
	totalSize := storedSize + bitmapHeaderSize
	blocksUsed := (totalSize + a.minBlockSize - 1) >> a.minBlockShift
	startIdx := blockOffset >> a.minBlockShift

	*(*uint32)(headerPtr) = 0

	if blocksUsed == 1 {
		a.bitmap[startIdx>>3] &^= 1 << (startIdx & 7)
		return
	}
	a.setBlocks(startIdx, blocksUsed, false)
}

// findFreeBit finds a single free block starting from startIdx.
// Optimized to scan uint64 words using TrailingZeros64.
func (a *BitmapAllocator) findFreeBit(startIdx int) int {
	bitmap := a.bitmap
	n := len(bitmap)
	byteIdx := startIdx >> 3
	bitIdx := startIdx & 7

	// Handle partial first byte
	if bitIdx != 0 && byteIdx < n {
		b := bitmap[byteIdx] | (byte(1<<bitIdx) - 1)
		if b != 0xFF {
			idx := byteIdx<<3 + bits.TrailingZeros8(^b)
			if idx < a.numBlocks {
				return idx
			}
			return -1
		}
		byteIdx++
	}

	// Scan 64-bit words
	for byteIdx+8 <= n {
		val := *(*uint64)(unsafe.Pointer(&bitmap[byteIdx]))
		if val != ^uint64(0) {
			idx := byteIdx<<3 + bits.TrailingZeros64(^val)
			if idx < a.numBlocks {
				return idx
			}
			return -1
		}
		byteIdx += 8
	}

	// Scan remaining bytes
	for ; byteIdx < n; byteIdx++ {
		if bitmap[byteIdx] != 0xFF {
			idx := byteIdx<<3 + bits.TrailingZeros8(^bitmap[byteIdx])
			if idx < a.numBlocks {
				return idx
			}
			return -1
		}
	}
	return -1
}

// findFreeRun finds blocksNeeded contiguous free blocks starting from startIdx.
// Returns -1 if not found before end of bitmap.
// Optimized to use word scanning with unaligned head/tail handling.
func (a *BitmapAllocator) findFreeRun(startIdx, blocksNeeded int) int {
	runStart := -1
	runLen := 0
	i := startIdx
	n := a.numBlocks

	// 1. Scan unaligned head
	for i < n && (i&63) != 0 {
		if a.isSet(i) {
			runStart = -1
			runLen = 0
		} else {
			if runStart == -1 {
				runStart = i
			}
			runLen++
			if runLen >= blocksNeeded {
				return runStart
			}
		}
		i++
	}

	if i >= n {
		return -1
	}

	// 2. Scan aligned 64-bit words
	byteIdx := i >> 3
	for i+64 <= n {
		val := *(*uint64)(unsafe.Pointer(&a.bitmap[byteIdx]))

		// All used
		if val == ^uint64(0) {
			runStart = -1
			runLen = 0
			i += 64
			byteIdx += 8
			continue
		}

		// All free
		if val == 0 {
			if runStart == -1 {
				runStart = i
			}
			runLen += 64
			if runLen >= blocksNeeded {
				return runStart
			}
			i += 64
			byteIdx += 8
			continue
		}

		// Mixed word: scan bits in register
		base := i
		for k := 0; k < 64; k++ {
			if (val>>k)&1 != 0 {
				runStart = -1
				runLen = 0
			} else {
				if runStart == -1 {
					runStart = base + k
				}
				runLen++
				if runLen >= blocksNeeded {
					return runStart
				}
			}
		}
		i += 64
		byteIdx += 8
	}

	// 3. Scan remaining tail
	for i < n {
		if a.isSet(i) {
			runStart = -1
			runLen = 0
		} else {
			if runStart == -1 {
				runStart = i
			}
			runLen++
			if runLen >= blocksNeeded {
				return runStart
			}
		}
		i++
	}

	return -1
}

// isSet returns true if block at idx is allocated.
func (a *BitmapAllocator) isSet(idx int) bool {
	return a.bitmap[idx>>3]&(1<<(idx&7)) != 0
}

// setBlocks marks count blocks starting at idx as used (set=true) or free (set=false).
func (a *BitmapAllocator) setBlocks(idx, count int, set bool) {
	if count == 0 {
		return
	}
	end := idx + count
	startByte := idx >> 3
	endByte := (end - 1) >> 3

	if startByte == endByte {
		// All bits in same byte
		mask := byte((1<<count)-1) << (idx & 7)
		if set {
			a.bitmap[startByte] |= mask
		} else {
			a.bitmap[startByte] &^= mask
		}
		return
	}

	// First byte: set bits from idx&7 to 7
	firstMask := byte(0xFF) << (idx & 7)
	if set {
		a.bitmap[startByte] |= firstMask
	} else {
		a.bitmap[startByte] &^= firstMask
	}

	// Middle bytes: set all bits
	if set {
		for i := startByte + 1; i < endByte; i++ {
			a.bitmap[i] = 0xFF
		}
	} else {
		for i := startByte + 1; i < endByte; i++ {
			a.bitmap[i] = 0
		}
	}

	// Last byte: set bits 0 to (end-1)&7
	lastMask := byte((1 << ((end-1)&7 + 1)) - 1)
	if set {
		a.bitmap[endByte] |= lastMask
	} else {
		a.bitmap[endByte] &^= lastMask
	}
}
