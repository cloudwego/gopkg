package malloc

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBitmapAllocator(t *testing.T) {
	tests := []struct {
		name    string
		size    int
		min     int
		max     int
		wantErr bool
	}{
		{"valid", 1024 * 1024, 4096, 64 * 1024, false},
		{"valid_min_eq_4k", 256 * 1024, 4096, 8192, false},
		{"valid_large_min", 1024 * 1024, 8192, 32768, false},
		{"min_lt_4096", 256 * 1024, 2048, 8192, true},
		{"min_not_mult_4096", 256 * 1024, 5000, 10000, true},
		{"max_le_min", 256 * 1024, 4096, 4096, true},
		{"max_not_mult_min", 256 * 1024, 4096, 10000, true},
		{"arena_too_small", 4096, 4096, 8192, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewBitmapAllocatorWithBlockSize(make([]byte, tt.size), tt.min, tt.max)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBitmapAllocFree(t *testing.T) {
	a := newTestBitmapAlloc(t, 1024*1024, 4096, 64*1024)

	// basic alloc
	b1 := a.Alloc(1024)
	require.NotNil(t, b1)
	assert.Equal(t, 1024, len(b1))

	// write to block
	for i := range b1 {
		b1[i] = byte(i)
	}

	// second alloc
	b2 := a.Alloc(8192)
	require.NotNil(t, b2)
	assert.False(t, bitmapOverlap(b1, b2))

	// free and reuse
	a.Free(b1)
	b3 := a.Alloc(2048)
	require.NotNil(t, b3)

	a.Free(b2)
	a.Free(b3)
}

func TestBitmapAllocSizes(t *testing.T) {
	a := newTestBitmapAlloc(t, 2*1024*1024, 4096, 128*1024)

	sizes := []int{1, 100, 1024, 4096, 8192, 16384, 32768, 65536}
	for _, sz := range sizes {
		b := a.Alloc(sz)
		require.NotNil(t, b, "size=%d", sz)
		assert.Equal(t, sz, len(b))
		a.Free(b)
	}
}

func TestBitmapAllocZeroNegative(t *testing.T) {
	a := newTestBitmapAlloc(t, 256*1024, 4096, 16*1024)
	assert.Nil(t, a.Alloc(0))
	assert.Nil(t, a.Alloc(-1))
}

func TestBitmapAllocTooLarge(t *testing.T) {
	a := newTestBitmapAlloc(t, 256*1024, 4096, 16*1024)
	assert.Nil(t, a.Alloc(16*1024)) // max - header exceeded
}

func TestBitmapAllocExhaustion(t *testing.T) {
	a := newTestBitmapAlloc(t, 256*1024, 4096, 64*1024)

	var blocks [][]byte
	for {
		b := a.Alloc(1024)
		if b == nil {
			break
		}
		blocks = append(blocks, b)
	}
	assert.Greater(t, len(blocks), 0)

	// should fail
	assert.Nil(t, a.Alloc(1))

	// free all
	for _, b := range blocks {
		a.Free(b)
	}

	// should work again
	b := a.Alloc(1024)
	require.NotNil(t, b)
	a.Free(b)
}

func TestBitmapNextFit(t *testing.T) {
	a := newTestBitmapAlloc(t, 256*1024, 4096, 16*1024)

	// Alloc several blocks
	b1 := a.Alloc(1024)
	b2 := a.Alloc(1024)
	b3 := a.Alloc(1024)
	require.NotNil(t, b1)
	require.NotNil(t, b2)
	require.NotNil(t, b3)

	// Free middle block
	a.Free(b2)

	// Next alloc should NOT reuse b2's space (next-fit continues forward)
	b4 := a.Alloc(1024)
	require.NotNil(t, b4)

	// Free first block, alloc again - should wrap around and find it
	a.Free(b1)
	a.Free(b3)
	a.Free(b4)
}

func TestBitmapContiguousAlloc(t *testing.T) {
	a := newTestBitmapAlloc(t, 512*1024, 4096, 64*1024)

	// Alloc large block spanning multiple min blocks
	b := a.Alloc(32 * 1024)
	require.NotNil(t, b)
	assert.Equal(t, 32*1024, len(b))

	// Write to entire block
	for i := range b {
		b[i] = byte(i)
	}

	a.Free(b)
}

func TestBitmapFreeInvalid(t *testing.T) {
	a := newTestBitmapAlloc(t, 256*1024, 4096, 16*1024)

	// outside arena
	assert.Panics(t, func() { a.Free(make([]byte, 1024)) })

	// nil/empty should not panic
	assert.NotPanics(t, func() { a.Free(nil) })
	assert.NotPanics(t, func() { a.Free([]byte{}) })

	// valid free
	b := a.Alloc(1024)
	assert.NotPanics(t, func() { a.Free(b) })

	// double free
	assert.Panics(t, func() { a.Free(b) })
}

func TestBitmapAvailable(t *testing.T) {
	a := newTestBitmapAlloc(t, 256*1024, 4096, 16*1024)
	initial := a.Available()
	assert.Greater(t, initial, 0)

	b := a.Alloc(4096)
	require.NotNil(t, b)
	assert.Less(t, a.Available(), initial)

	a.Free(b)
	assert.Equal(t, initial, a.Available())
}

func TestBitmapReset(t *testing.T) {
	a := newTestBitmapAlloc(t, 256*1024, 4096, 16*1024)
	initial := a.Available()

	// Alloc several blocks
	var blocks [][]byte
	for i := 0; i < 10; i++ {
		b := a.Alloc(1024)
		require.NotNil(t, b)
		blocks = append(blocks, b)
	}
	assert.Less(t, a.Available(), initial)

	// Reset should restore all memory
	a.Reset()
	assert.Equal(t, initial, a.Available())

	// Should be able to alloc again
	b := a.Alloc(1024)
	require.NotNil(t, b)
	a.Free(b)
}

func TestBitmapIsValidOffset(t *testing.T) {
	a := newTestBitmapAlloc(t, 256*1024, 4096, 16*1024)

	t.Run("ValidFromAlloc", func(t *testing.T) {
		b := a.Alloc(1024)
		require.NotNil(t, b)
		dataPtr := uintptr(unsafe.Pointer(&b[0]))
		blocksPtr := uintptr(a.blocksStart)
		dataOffset := int(dataPtr - blocksPtr)

		assert.True(t, a.IsValidOffset(dataOffset))
		a.Free(b)
	})

	t.Run("ValidAligned", func(t *testing.T) {
		// Valid: block_offset=0, data_offset=8
		assert.True(t, a.IsValidOffset(bitmapHeaderSize))
		// Valid: block_offset=4096, data_offset=4104
		assert.True(t, a.IsValidOffset(4096+bitmapHeaderSize))
	})

	t.Run("InvalidNegative", func(t *testing.T) {
		assert.False(t, a.IsValidOffset(-1))
		assert.False(t, a.IsValidOffset(-100))
	})

	t.Run("InvalidOutOfBounds", func(t *testing.T) {
		maxOffset := a.numBlocks << a.minBlockShift
		assert.False(t, a.IsValidOffset(maxOffset+bitmapHeaderSize))
		assert.False(t, a.IsValidOffset(maxOffset*2))
	})

	t.Run("InvalidMisaligned", func(t *testing.T) {
		// Offset 0 means block_offset=-8 which is invalid
		assert.False(t, a.IsValidOffset(0))
		// Offset 100 means block_offset=92 which is not aligned
		assert.False(t, a.IsValidOffset(100))
		// Offset 4104 means block_offset=4096 which IS aligned
		assert.True(t, a.IsValidOffset(4096+bitmapHeaderSize))
		// Offset 4105 means block_offset=4097 which is not aligned
		assert.False(t, a.IsValidOffset(4096+bitmapHeaderSize+1))
	})
}

func TestBitmapFreeAt(t *testing.T) {
	a := newTestBitmapAlloc(t, 256*1024, 4096, 16*1024)

	t.Run("Basic", func(t *testing.T) {
		initial := a.Available()
		b := a.Alloc(1024)
		require.NotNil(t, b)

		dataPtr := uintptr(unsafe.Pointer(&b[0]))
		blocksPtr := uintptr(a.blocksStart)
		dataOffset := int(dataPtr - blocksPtr)

		a.FreeAt(dataOffset)
		assert.Equal(t, initial, a.Available())
	})

	t.Run("MultiBlock", func(t *testing.T) {
		initial := a.Available()
		b := a.Alloc(8192) // spans 3 blocks with header
		require.NotNil(t, b)

		dataPtr := uintptr(unsafe.Pointer(&b[0]))
		blocksPtr := uintptr(a.blocksStart)
		dataOffset := int(dataPtr - blocksPtr)

		a.FreeAt(dataOffset)
		assert.Equal(t, initial, a.Available())
	})

	t.Run("InvalidOffset", func(t *testing.T) {
		assert.Panics(t, func() { a.FreeAt(-100) })
		assert.Panics(t, func() { a.FreeAt(a.numBlocks << a.minBlockShift) })
	})

	t.Run("MisalignedOffset", func(t *testing.T) {
		assert.Panics(t, func() { a.FreeAt(100) })
	})

	t.Run("DoubleFree", func(t *testing.T) {
		b := a.Alloc(1024)
		require.NotNil(t, b)

		dataPtr := uintptr(unsafe.Pointer(&b[0]))
		blocksPtr := uintptr(a.blocksStart)
		dataOffset := int(dataPtr - blocksPtr)

		a.FreeAt(dataOffset)
		assert.Panics(t, func() { a.FreeAt(dataOffset) })
	})
}

// helpers

func newTestBitmapAlloc(t *testing.T, size, min, max int) *BitmapAllocator {
	t.Helper()
	a, err := NewBitmapAllocatorWithBlockSize(make([]byte, size), min, max)
	require.NoError(t, err)
	return a
}

func bitmapOverlap(a, b []byte) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	aStart := uintptr(*(*unsafe.Pointer)(unsafe.Pointer(&a)))
	aEnd := aStart + uintptr(len(a))
	bStart := uintptr(*(*unsafe.Pointer)(unsafe.Pointer(&b)))
	bEnd := bStart + uintptr(len(b))
	return !(aEnd <= bStart || bEnd <= aStart)
}

// benchmarks

func BenchmarkBitmapAlloc(b *testing.B) {
	a, _ := NewBitmapAllocatorWithBlockSize(make([]byte, 16*1024*1024), 4096, 64*1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		block := a.Alloc(4096)
		if block != nil {
			a.Free(block)
		}
	}
}

func BenchmarkBitmapAllocContiguous(b *testing.B) {
	a, _ := NewBitmapAllocatorWithBlockSize(make([]byte, 16*1024*1024), 4096, 64*1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		block := a.Alloc(32 * 1024)
		if block != nil {
			a.Free(block)
		}
	}
}

func BenchmarkBitmapAllocSizes(b *testing.B) {
	a, _ := NewBitmapAllocatorWithBlockSize(make([]byte, 16*1024*1024), 4096, 64*1024)
	sizes := []int{4096, 8192, 16384, 32768}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		block := a.Alloc(sizes[i&3])
		if block != nil {
			a.Free(block)
		}
	}
}

func BenchmarkBitmapAllocParallel(b *testing.B) {
	a, _ := NewBitmapAllocatorWithBlockSize(make([]byte, 64*1024*1024), 4096, 64*1024)
	blocks := make([][]byte, 64)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx := i & 63
		if blocks[idx] != nil {
			a.Free(blocks[idx])
		}
		blocks[idx] = a.Alloc(4096)
	}
}

func BenchmarkBitmapNextFit(b *testing.B) {
	a, _ := NewBitmapAllocatorWithBlockSize(make([]byte, 16*1024*1024), 4096, 64*1024)
	// Pre-fragment: alloc every other block
	var held [][]byte
	for i := 0; i < 100; i++ {
		b1 := a.Alloc(4096)
		b2 := a.Alloc(4096)
		if b1 != nil && b2 != nil {
			held = append(held, b1)
			a.Free(b2)
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		block := a.Alloc(4096)
		if block != nil {
			a.Free(block)
		}
	}
	b.StopTimer()
	for _, blk := range held {
		a.Free(blk)
	}
}
