package malloc

import (
	"math/rand"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBuddyAllocator(t *testing.T) {
	tests := []struct {
		size    int
		wantErr bool
	}{
		{512 * 1024, false},      // min valid
		{1024 * 1024, false},     // 1MB
		{4 * 1024 * 1024, false}, // 4MB
		{256 * 1024, true},       // too small
		{768 * 1024, true},       // not multiple
	}
	for _, tt := range tests {
		_, err := NewBuddyAllocator(make([]byte, tt.size))
		if tt.wantErr {
			assert.Error(t, err, "size=%d", tt.size)
		} else {
			assert.NoError(t, err, "size=%d", tt.size)
		}
	}
}

func TestNewBuddyAllocatorWithBlockSize(t *testing.T) {
	tests := []struct {
		name    string
		size    int
		min     int
		max     int
		wantErr bool
	}{
		{"valid_custom", 64 * 1024, 1024, 64 * 1024, false},
		{"valid_same_min_max", 4096, 4096, 4096, false},
		{"valid_multi_root", 128 * 1024, 1024, 64 * 1024, false},
		{"min_not_pow2", 64 * 1024, 1000, 64 * 1024, true},
		{"max_not_pow2", 64 * 1024, 1024, 60000, true},
		{"min_gt_max", 64 * 1024, 8192, 4096, true},
		{"min_le_header", 64 * 1024, 8, 64 * 1024, true},
		{"arena_not_multiple", 100 * 1024, 1024, 64 * 1024, true},
		{"arena_too_small", 32 * 1024, 1024, 64 * 1024, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewBuddyAllocatorWithBlockSize(make([]byte, tt.size), tt.min, tt.max)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBuddyAllocatorWithCustomBlockSize(t *testing.T) {
	// 64KB arena with 1KB min, 16KB max blocks
	a, err := NewBuddyAllocatorWithBlockSize(make([]byte, 64*1024), 1024, 16*1024)
	require.NoError(t, err)

	// alloc small block
	b1 := a.Alloc(500)
	require.NotNil(t, b1)
	assert.Equal(t, 500, len(b1))
	assert.Equal(t, 1024-headerSize, cap(b1))

	// alloc large block
	b2 := a.Alloc(8000)
	require.NotNil(t, b2)
	assert.Equal(t, 8000, len(b2))
	assert.False(t, overlap(b1, b2))

	// free and realloc
	a.Free(b1)
	a.Free(b2)

	// should be able to alloc max size
	b3 := a.Alloc(16*1024 - headerSize)
	require.NotNil(t, b3)
	assert.Equal(t, 16*1024-headerSize, len(b3))
}

func TestAllocFree(t *testing.T) {
	a := newTestBuddyAllocator(t, 512*1024)

	// basic alloc
	b1 := a.Alloc(1024)
	require.NotNil(t, b1)
	assert.Equal(t, 1024, len(b1))
	assert.Equal(t, DefaultMinBlockSize-headerSize, cap(b1))

	// write to block
	for i := range b1 {
		b1[i] = byte(i)
	}

	// second alloc
	b2 := a.Alloc(8192)
	require.NotNil(t, b2)
	assert.False(t, overlap(b1, b2))

	// free and reuse
	a.Free(b1)
	b3 := a.Alloc(4096)
	require.NotNil(t, b3)
}

func TestAllocSizes(t *testing.T) {
	a := newTestBuddyAllocator(t, 1024*1024)

	sizes := []int{1, 100, 1024, 4096, 8192, 16384, 32768, 65536, 131072}
	for _, sz := range sizes {
		b := a.Alloc(sz)
		require.NotNil(t, b, "size=%d", sz)
		assert.GreaterOrEqual(t, len(b), sz)
		a.Free(b)
	}
}

func TestAllocZero(t *testing.T) {
	a := newTestBuddyAllocator(t, 512*1024)
	assert.Nil(t, a.Alloc(0))
	assert.Nil(t, a.Alloc(-1))
}

func TestAllocTooLarge(t *testing.T) {
	a := newTestBuddyAllocator(t, 512*1024)
	assert.Nil(t, a.Alloc(1024*1024))
}

func TestAllocCoalesceFails(t *testing.T) {
	a := newTestBuddyAllocator(t, 1024*1024)
	clearBuddyFreeLists(a)

	// add non-buddy nodes at order 0 (can't merge)
	a.freeLists[0] = append(a.freeLists[0], 3, 5, 7) // no buddies
	a.needsCoalesce = true

	// request order 1 (16KB) - coalesce will be attempted but fail
	b := a.Alloc(16384)
	assert.Nil(t, b)
	assert.False(t, a.needsCoalesce) // should be cleared after failed coalesce
}

func TestAllocExhaustion(t *testing.T) {
	a := newTestBuddyAllocator(t, 512*1024)

	// exhaust memory
	var blocks [][]byte
	for {
		b := a.Alloc(1024) // Fits in 8KB block
		if b == nil {
			break
		}
		blocks = append(blocks, b)
	}
	assert.Equal(t, 64, len(blocks)) // 512KB / 8KB

	// should fail
	assert.Nil(t, a.Alloc(1))

	// free all and alloc large
	for _, b := range blocks {
		a.Free(b)
	}
	large := a.Alloc(DefaultMaxBlockSize / 2)
	require.NotNil(t, large)
	assert.Equal(t, DefaultMaxBlockSize/2, len(large))
}

func TestSplitting(t *testing.T) {
	a := newTestBuddyAllocator(t, 512*1024)

	b1 := a.Alloc(1024)
	require.NotNil(t, b1)
	assert.Equal(t, 1024, len(b1))
	assert.Equal(t, DefaultMinBlockSize-headerSize, cap(b1))

	b2 := a.Alloc(DefaultMaxBlockSize / 4)
	require.NotNil(t, b2)
	assert.Equal(t, DefaultMaxBlockSize/4, len(b2))
	assert.GreaterOrEqual(t, cap(b2), DefaultMaxBlockSize/4)

	a.Free(b1)
	a.Free(b2)
}

func TestCoalescing(t *testing.T) {
	t.Run("Basic", func(t *testing.T) {
		a := newTestBuddyAllocator(t, 512*1024)
		b1 := a.Alloc(8192)
		b2 := a.Alloc(8192)
		a.Alloc(8192) // b3
		a.Alloc(8192) // b4

		a.Free(b2)
		a.Free(b1) // should coalesce

		large := a.Alloc(16384)
		require.NotNil(t, large)
		assert.Equal(t, 16384, len(large))
	})

	t.Run("CoalesceUntil", func(t *testing.T) {
		tests := []struct {
			name   string
			nodes  []int // offsets
			target int
			want   int
		}{
			{"TwoBuddies", []int{0, 8192}, 1, 1},
			{"FourNodes", []int{0, 8192, 16384, 24576}, 2, 2},
			{"EightNodes", []int{0, 8192, 16384, 24576, 32768, 40960, 49152, 57344}, 3, 3},
			{"NoBuddies", []int{0, 16384, 32768, 49152}, 1, -1},
			{"SingleNode", []int{16384}, 1, -1},
			{"RootCantMerge", []int{0, 512 * 1024}, 1, -1},
			{"PartialBuddies", []int{0, 8192, 32768}, 2, -1},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				a := newTestBuddyAllocator(t, 1024*1024)
				clearBuddyFreeLists(a)
				a.freeLists[0] = append(a.freeLists[0], tt.nodes...)
				assert.Equal(t, tt.want, a.CoalesceUntil(tt.target))
			})
		}
	})

	t.Run("AlreadyAvailable", func(t *testing.T) {
		a := newTestBuddyAllocator(t, 1024*1024)
		clearBuddyFreeLists(a)
		a.freeLists[2] = append(a.freeLists[2], 0)
		assert.Equal(t, 2, a.CoalesceUntil(2))
	})

	t.Run("HigherOrderAvailable", func(t *testing.T) {
		a := newTestBuddyAllocator(t, 1024*1024)
		clearBuddyFreeLists(a)
		a.freeLists[3] = append(a.freeLists[3], 0)
		assert.Equal(t, 3, a.CoalesceUntil(1))
	})
}

func TestIsValidOffset(t *testing.T) {
	a := newTestBuddyAllocator(t, 512*1024) // 512KB with 8KB min block

	t.Run("ValidFromAlloc", func(t *testing.T) {
		b := a.Alloc(1024)
		require.NotNil(t, b)
		// Get the data offset (pointer to returned slice)
		dataPtr := uintptr(unsafe.Pointer(&b[0]))
		arenaPtr := uintptr(a.arenaStart)
		dataOffset := int(dataPtr - arenaPtr)

		assert.True(t, a.IsValidOffset(dataOffset))
		a.Free(b)
	})

	t.Run("ValidAligned", func(t *testing.T) {
		// Valid: block_offset=0, data_offset=headerSize(8)
		assert.True(t, a.IsValidOffset(headerSize))
		// Valid: block_offset=8192, data_offset=8192+8=8200
		assert.True(t, a.IsValidOffset(8192+headerSize))
		// Valid: block_offset=16384, data_offset=16384+8
		assert.True(t, a.IsValidOffset(16384+headerSize))
	})

	t.Run("InvalidNegative", func(t *testing.T) {
		assert.False(t, a.IsValidOffset(-1))
		assert.False(t, a.IsValidOffset(-100))
	})

	t.Run("InvalidOutOfBounds", func(t *testing.T) {
		assert.False(t, a.IsValidOffset(512 * 1024))       // exactly at end
		assert.False(t, a.IsValidOffset(512*1024 + 100))   // past end
		assert.False(t, a.IsValidOffset(1024 * 1024 * 10)) // way past end
	})

	t.Run("InvalidMisaligned", func(t *testing.T) {
		// Offset 0 means block_offset=-8 which is invalid
		assert.False(t, a.IsValidOffset(0))
		// Offset 100 means block_offset=92 which is not aligned to 8KB
		assert.False(t, a.IsValidOffset(100))
		// Offset 1000 means block_offset=992 which is not aligned
		assert.False(t, a.IsValidOffset(1000))
		// Offset 8200 means block_offset=8192 which IS aligned - this should be valid
		assert.True(t, a.IsValidOffset(8200))
		// Offset 8201 means block_offset=8193 which is not aligned
		assert.False(t, a.IsValidOffset(8201))
	})

	t.Run("CustomBlockSize", func(t *testing.T) {
		// 64KB arena with 1KB min block
		a2, err := NewBuddyAllocatorWithBlockSize(make([]byte, 64*1024), 1024, 64*1024)
		require.NoError(t, err)

		// Valid: block_offset=0, data_offset=8
		assert.True(t, a2.IsValidOffset(headerSize))
		// Valid: block_offset=1024, data_offset=1032
		assert.True(t, a2.IsValidOffset(1024+headerSize))
		// Invalid: block_offset=512 not aligned to 1KB
		assert.False(t, a2.IsValidOffset(512+headerSize))
	})
}

func TestFreeInvalid(t *testing.T) {
	a := newTestBuddyAllocator(t, 512*1024)
	arena := make([]byte, 512*1024)

	tests := []struct {
		name  string
		block []byte
	}{
		{"outside", make([]byte, 8192)},
		{"misaligned", arena[100:8292]},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Panics(t, func() { a.Free(tt.block) })
		})
	}

	// nil/empty should not panic
	assert.NotPanics(t, func() { a.Free(nil) })
	assert.NotPanics(t, func() { a.Free([]byte{}) })

	// valid free should not panic
	b := a.Alloc(8192)
	assert.NotPanics(t, func() { a.Free(b) })

	// double free should panic
	assert.Panics(t, func() { a.Free(b) })

	// corrupted size (cap < requested size) should panic
	b2 := a.Alloc(100)
	assert.Panics(t, func() { a.Free(b2[:50:60]) })
}

func TestAvailableAfterRandomAllocFree(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	a := newTestBuddyAllocator(t, 4*1024*1024) // 4MB
	initial := a.Available()

	var blocks [][]byte

	sizes := []int{100, 512, 1024, 4096, 8192, 16384, 32768, 65536}

	// Random alloc/free operations
	for i := 0; i < 100000; i++ {
		if len(blocks) == 0 || rng.Intn(3) != 0 {
			// Alloc random size
			sz := sizes[rng.Intn(len(sizes))]
			b := a.Alloc(sz)
			if b != nil {
				blocks = append(blocks, b)
			}
		} else {
			// Free random block
			idx := rng.Intn(len(blocks))
			a.Free(blocks[idx])
			blocks[idx] = blocks[len(blocks)-1]
			blocks = blocks[:len(blocks)-1]
		}
	}

	// Free all remaining
	for _, b := range blocks {
		a.Free(b)
	}

	// Trigger coalescing (lazy coalesce needs a large alloc request)
	a.CoalesceUntil(a.maxBlockOrder)

	assert.Equal(t, initial, a.Available())
}

// helpers

func newTestBuddyAllocator(t *testing.T, size int) *BuddyAllocator {
	t.Helper()
	a, err := NewBuddyAllocator(make([]byte, size))
	require.NoError(t, err)
	return a
}

func clearBuddyFreeLists(a *BuddyAllocator) {
	for i := range a.freeLists {
		a.freeLists[i] = a.freeLists[i][:0]
	}
}

func overlap(a, b []byte) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	aStart := uintptr(unsafe.Pointer(&a[0]))
	aEnd := aStart + uintptr(len(a))
	bStart := uintptr(unsafe.Pointer(&b[0]))
	bEnd := bStart + uintptr(len(b))
	return !(aEnd <= bStart || bEnd <= aStart)
}

// benchmarks

func BenchmarkAlloc(b *testing.B) {
	a, _ := NewBuddyAllocator(make([]byte, 16*1024*1024))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		block := a.Alloc(8192)
		if block != nil {
			a.Free(block)
		}
	}
}

func BenchmarkAllocSizes(b *testing.B) {
	a, _ := NewBuddyAllocator(make([]byte, 16*1024*1024))
	sizes := []int{1024, 8192, 32768, 131072}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		block := a.Alloc(sizes[i%len(sizes)])
		if block != nil {
			a.Free(block)
		}
	}
}

func BenchmarkCoalescing(b *testing.B) {
	benchmarks := []struct {
		name    string
		offsets []int
		target  int
	}{
		{"16KB", []int{0, 8192}, 1},
		{"64KB", []int{0, 8192, 16384, 24576, 32768, 40960, 49152, 57344}, 3},
		{"256KB", genOffsets(0, 32, 8192), 5},
		{"512KB", genOffsets(0, 64, 8192), 6},
	}
	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			a, _ := NewBuddyAllocator(make([]byte, 16*1024*1024))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				clearBuddyFreeLists(a)
				a.freeLists[0] = append(a.freeLists[0], bm.offsets...)
				a.needsCoalesce = true
				a.CoalesceUntil(bm.target)
			}
		})
	}
}

func genOffsets(start, count, stride int) []int {
	offsets := make([]int, count)
	for i := 0; i < count; i++ {
		offsets[i] = start + i*stride
	}
	return offsets
}
