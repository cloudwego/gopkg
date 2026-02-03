package malloc

import (
	"fmt"
	"math/bits"
	"unsafe"
)

const (
	// headerSize is the size of the header added to each allocation.
	headerSize = 8

	// magic is the magic number checked to detect double-free/invalid blocks.
	magic uint32 = 0xBADF00D

	// DefaultMinBlockSize is the default minimum block size (8KB).
	DefaultMinBlockSize = 8 * 1024 // 8KB = 2^13

	// DefaultMaxBlockSize is the default maximum block size (512KB).
	DefaultMaxBlockSize = 512 * 1024 // 512KB = 2^19
)

// BuddyAllocator is the main buddy system allocator.
type BuddyAllocator struct {
	// arena is the underlying memory slab we are managing.
	arena []byte

	// arenaStart is a cached pointer to the start of the arena.
	// Used for fast offset calculations in Free().
	arenaStart unsafe.Pointer

	// freeLists holds slices of free block offsets for each order.
	// freeLists[0] is for minBlockSize blocks (order 0).
	// freeLists[maxBlockOrder] is for maxBlockSize blocks (the largest).
	freeLists [][]int

	// needsCoalesce is a hint that adjacent free blocks may exist that can be merged.
	// Set to true on Free() of non-max-order blocks, cleared when coalescing fails.
	needsCoalesce bool

	// minBlockSize is the minimum block size.
	minBlockSize int
	// minBlockShift is log2(minBlockSize).
	minBlockShift int
	// maxBlockSize is the maximum block size.
	maxBlockSize int
	// maxBlockOrder is log2(maxBlockSize) - log2(minBlockSize).
	maxBlockOrder int
}

// NewBuddyAllocator creates a new buddy allocator with default block sizes (8KB min, 512KB max).
// The arena's size MUST be a multiple of maxBlockSize.
func NewBuddyAllocator(arena []byte) (*BuddyAllocator, error) {
	return NewBuddyAllocatorWithBlockSize(arena, DefaultMinBlockSize, DefaultMaxBlockSize)
}

// NewBuddyAllocatorWithBlockSize creates a new buddy allocator with custom block sizes.
// Both minBlock and maxBlock must be powers of two, and minBlock <= maxBlock.
// The arena's size MUST be a multiple of maxBlock.
func NewBuddyAllocatorWithBlockSize(arena []byte, minBlock, maxBlock int) (*BuddyAllocator, error) {
	// Validate block sizes are powers of two
	if minBlock <= 0 || (minBlock&(minBlock-1)) != 0 {
		return nil, fmt.Errorf("minBlockSize must be a power of two, got %d", minBlock)
	}
	if maxBlock <= 0 || (maxBlock&(maxBlock-1)) != 0 {
		return nil, fmt.Errorf("maxBlockSize must be a power of two, got %d", maxBlock)
	}
	if minBlock > maxBlock {
		return nil, fmt.Errorf("minBlockSize (%d) must be <= maxBlockSize (%d)", minBlock, maxBlock)
	}
	if minBlock <= headerSize {
		return nil, fmt.Errorf("minBlockSize must be > headerSize (%d), got %d", headerSize, minBlock)
	}

	totalSize := len(arena)

	// Validate arena size: must be a multiple of maxBlock and at least maxBlock
	if totalSize < maxBlock || totalSize%maxBlock != 0 {
		return nil, fmt.Errorf("arena size must be a multiple of %d bytes (%dKB) and >= %dKB, got %d",
			maxBlock, maxBlock>>10, maxBlock>>10, totalSize)
	}

	minShift := bits.TrailingZeros(uint(minBlock))
	maxShift := bits.TrailingZeros(uint(maxBlock))
	maxOrder := maxShift - minShift

	// Calculate number of root blocks
	numRootBlocks := totalSize / maxBlock

	// Initialize allocator
	a := &BuddyAllocator{
		arena:         arena,
		arenaStart:    unsafe.Pointer(&arena[0]),
		minBlockSize:  minBlock,
		minBlockShift: minShift,
		maxBlockSize:  maxBlock,
		maxBlockOrder: maxOrder,
		freeLists:     make([][]int, maxOrder+1),
	}

	// Initialize all free lists with pre-allocated capacity.
	// Lower orders can hold more blocks (each split doubles the count),
	// so capacity = 2^(maxOrder-i). Capped at 64 to avoid over-allocation.
	for i := 0; i < maxOrder; i++ {
		capacity := 1 << (maxOrder - i)
		if capacity > 64 {
			capacity = 64
		}
		a.freeLists[i] = make([]int, 0, capacity)
	}

	// Initialize root block list with exact capacity
	a.freeLists[maxOrder] = make([]int, 0, numRootBlocks)

	// Add all root blocks to the free list for maxOrder
	for i := 0; i < numRootBlocks; i++ {
		a.freeLists[maxOrder] = append(a.freeLists[maxOrder], i*maxBlock)
	}

	return a, nil
}

// Alloc allocates a block of memory of at least `size` bytes.
// It returns a slice of the allocated memory, or nil if no
// sufficiently large block is available.
func (a *BuddyAllocator) Alloc(size int) []byte {
	if size <= 0 || size > a.maxBlockSize-headerSize {
		return nil
	}
	order := a.getOrderForSize(size + headerSize)

	// Fast path: exact order match
	if freeList := a.freeLists[order]; len(freeList) > 0 {
		n := len(freeList) - 1
		offset := freeList[n]
		a.freeLists[order] = freeList[:n]

		// Write header: [4 bytes magic][4 bytes size]
		ptr := unsafe.Add(a.arenaStart, offset)
		*(*uint32)(ptr) = magic
		*(*uint32)(unsafe.Add(ptr, 4)) = uint32(size)

		blockSize := a.minBlockSize << order
		return unsafe.Slice((*byte)(unsafe.Add(ptr, headerSize)), blockSize-headerSize)[:size]
	}

	return a.allocSlow(size, order)
}

func (a *BuddyAllocator) allocSlow(size, order int) []byte {
	// Find higher order block
	foundOrder := -1
	for o := order + 1; o <= a.maxBlockOrder; o++ {
		if len(a.freeLists[o]) > 0 {
			foundOrder = o
			break
		}
	}

	// No block available - try coalescing
	if foundOrder == -1 {
		if !a.needsCoalesce {
			return nil
		}
		foundOrder = a.CoalesceUntil(order)
		if foundOrder == -1 {
			a.needsCoalesce = false
			return nil
		}
	}

	// Pop block from free list
	freeList := a.freeLists[foundOrder]
	n := len(freeList) - 1
	offset := freeList[n]
	a.freeLists[foundOrder] = freeList[:n]

	// Split until we reach required order.
	// A block is split into two buddies: the left half retains the original offset,
	// and the right half's offset is added to the free list of the new (lower) order.
	for foundOrder > order {
		foundOrder--
		right := offset + (a.minBlockSize << foundOrder)
		a.freeLists[foundOrder] = append(a.freeLists[foundOrder], right)
	}

	// Write header: [4 bytes magic][4 bytes size]
	ptr := unsafe.Add(a.arenaStart, offset)
	*(*uint32)(ptr) = magic
	*(*uint32)(unsafe.Add(ptr, 4)) = uint32(size)

	blockSize := a.minBlockSize << order
	return unsafe.Slice((*byte)(unsafe.Add(ptr, headerSize)), blockSize-headerSize)[:size]
}

// Free returns a block of memory to the allocator.
// Panics if the block doesn't belong to this allocator.
// Uses lazy coalescing - blocks are marked free but not merged until needed.
//
// IMPORTANT: The block must be the original slice returned by Alloc.
// Do not reslice (e.g., block[n:]) before calling Free, as this corrupts
// the offset calculation and may lead to memory corruption or panics.
func (a *BuddyAllocator) Free(block []byte) {
	size := cap(block)
	if size == 0 {
		return
	}
	if size > a.maxBlockSize {
		panic("buddy: invalid block size")
	}
	// Calculate the original block offset by subtracting headerSize from the slice pointer.
	// Use slice header directly to avoid panic on zero-length slices.
	dataPtr := *(*uintptr)(unsafe.Pointer(&block))
	offset := int(dataPtr-uintptr(a.arenaStart)) - headerSize
	if offset < 0 || offset >= len(a.arena) {
		panic("buddy: block not in arena")
	}

	// Check magic and stored size
	headerPtr := unsafe.Add(a.arenaStart, offset)
	magicPtr := (*uint32)(headerPtr)
	if *magicPtr != magic {
		panic("buddy: double free or invalid block")
	}

	// Verify size matches what we expect
	storedSize := *(*uint32)(unsafe.Add(headerPtr, 4))
	if int(storedSize) > size {
		panic("buddy: corrupted size")
	}

	totalBlockSize := size + headerSize
	order := a.getOrderForSize(totalBlockSize)
	// Verify block is aligned to its buddy boundary.
	// Buddy blocks of size 2^N must start at an offset that is a multiple of 2^N.
	if offset&(totalBlockSize-1) != 0 {
		panic("buddy: misaligned block")
	}

	*magicPtr = 0
	a.freeLists[order] = append(a.freeLists[order], offset)
	if order < a.maxBlockOrder {
		a.needsCoalesce = true
	}
}

// IsValidOffset checks if the given data offset could be a valid allocation start.
// It validates bounds and alignment without checking the magic (allocation state).
// Use this for pre-validation before FreeAt to avoid panics from untrusted input.
func (a *BuddyAllocator) IsValidOffset(dataOffset int) bool {
	blockOffset := dataOffset - headerSize
	if blockOffset < 0 || blockOffset >= len(a.arena) {
		return false
	}
	// Block must be aligned to minBlockSize
	return blockOffset&(a.minBlockSize-1) == 0
}

// FreeAt returns a block at the given data offset to the allocator.
// The offset should be the same value returned by Alloc (pointing to user data).
// Panics if the offset is invalid or the block doesn't belong to this allocator.
func (a *BuddyAllocator) FreeAt(dataOffset int) {
	offset := dataOffset - headerSize
	if offset < 0 || offset >= len(a.arena) {
		panic("buddy: offset out of range")
	}

	headerPtr := unsafe.Add(a.arenaStart, offset)
	magicPtr := (*uint32)(headerPtr)
	if *magicPtr != magic {
		panic("buddy: double free or invalid block")
	}

	storedSize := int(*(*uint32)(unsafe.Add(headerPtr, 4)))
	totalBlockSize := storedSize + headerSize
	// Round up to power of two
	order := a.getOrderForSize(totalBlockSize)
	blockSize := a.minBlockSize << order

	if offset&(blockSize-1) != 0 {
		panic("buddy: misaligned block")
	}

	*magicPtr = 0
	a.freeLists[order] = append(a.freeLists[order], offset)
	if order < a.maxBlockOrder {
		a.needsCoalesce = true
	}
}

// Available returns the total free bytes available for allocation.
func (a *BuddyAllocator) Available() int {
	total := 0
	for order, freeList := range a.freeLists {
		blockSize := a.minBlockSize << order
		total += len(freeList) * (blockSize - headerSize)
	}
	return total
}

// CoalesceUntil merges adjacent free buddy blocks until we have a block >= targetOrder.
// Returns the order of a suitable block found, or -1 if none available.
func (a *BuddyAllocator) CoalesceUntil(targetOrder int) int {
	for o := targetOrder; o <= a.maxBlockOrder; o++ {
		if len(a.freeLists[o]) > 0 {
			return o
		}
	}

	// Coalesce from order 0 up to targetOrder-1.
	// Merging at lower orders creates blocks that can be merged at higher orders.
	for order := 0; order < targetOrder; order++ {
		freeList := a.freeLists[order]
		listLen := len(freeList)
		if listLen < 2 {
			continue
		}

		// Sort so buddies are adjacent (they differ by exactly blockSize).
		// Check if already sorted first (common case).
		sorted := true
		for i := 1; i < listLen; i++ {
			if freeList[i] < freeList[i-1] {
				sorted = false
				break
			}
		}
		if !sorted {
			// Insertion sort: O(n²) but fast for small/nearly-sorted slices,
			// which is typical for free lists.
			for i := 1; i < listLen; i++ {
				for j := i; j > 0 && freeList[j] < freeList[j-1]; j-- {
					freeList[j], freeList[j-1] = freeList[j-1], freeList[j]
				}
			}
		}

		blockSize := a.minBlockSize << order
		n := 0 // write index for remaining blocks

		for i := 0; i < listLen; {
			offset := freeList[i]
			// Buddy is offset ^ blockSize. When sorted, buddy is always offset + blockSize
			// for the left buddy (offset has 0 in the blockSize bit position).
			if i+1 < listLen && freeList[i+1] == offset^blockSize {
				// Found buddy pair, merge them.
				a.freeLists[order+1] = append(a.freeLists[order+1], offset&^blockSize)
				i += 2
			} else {
				freeList[n] = offset
				n++
				i++
			}
		}
		a.freeLists[order] = freeList[:n]
	}

	// Check what we achieved
	for o := targetOrder; o <= a.maxBlockOrder; o++ {
		if len(a.freeLists[o]) > 0 {
			return o
		}
	}
	return -1
}

// Reset clears all allocations and returns the allocator to its initial state.
func (a *BuddyAllocator) Reset() {
	// Clear all free lists except maxOrder
	for i := 0; i < a.maxBlockOrder; i++ {
		a.freeLists[i] = a.freeLists[i][:0]
	}

	// Reset root blocks
	numRoots := len(a.arena) / a.maxBlockSize
	a.freeLists[a.maxBlockOrder] = a.freeLists[a.maxBlockOrder][:0]
	for i := 0; i < numRoots; i++ {
		a.freeLists[a.maxBlockOrder] = append(a.freeLists[a.maxBlockOrder], i*a.maxBlockSize)
	}

	a.needsCoalesce = false
}

// getOrderForSize calculates the smallest order that can fit the given size.
// It uses bits.Len to find the smallest power of two that satisfies the request.
func (a *BuddyAllocator) getOrderForSize(size int) int {
	if size <= a.minBlockSize {
		return 0
	}
	return bits.Len(uint(size-1)) - a.minBlockShift
}
