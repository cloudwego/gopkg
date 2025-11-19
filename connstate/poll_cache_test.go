// Copyright 2025 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package connstate

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPollCacheAlloc(t *testing.T) {
	cache := &pollCache{}

	// Test initial allocation
	op1 := cache.alloc()
	require.NotNil(t, op1)
	assert.GreaterOrEqual(t, op1.index, int32(0))
	assert.Equal(t, int(0), op1.fd)

	// Test multiple allocations
	op2 := cache.alloc()
	require.NotNil(t, op2)
	assert.Equal(t, int(0), op2.fd)

	// Test that allocated operators are different
	assert.NotEqual(t, op1, op2)

	// Verify they both have valid indices
	assert.GreaterOrEqual(t, op1.index, int32(0))
	assert.GreaterOrEqual(t, op2.index, int32(0))
}

func TestPollCacheAllocReuse(t *testing.T) {
	cache := &pollCache{}

	// Allocate all operators
	var ops []*fdOperator
	for i := 0; i < 10; i++ {
		op := cache.alloc()
		require.NotNil(t, op)
		ops = append(ops, op)
	}

	// Mark some as freeable
	for i := 0; i < 5; i++ {
		cache.freeable(ops[i])
	}

	// Set freeack to trigger cleanup
	cache.free()

	// Allocate again, should reuse freed operators
	reusedOp := cache.alloc()
	require.NotNil(t, reusedOp)

	// The reused operator should have a high index (from cache)
	assert.GreaterOrEqual(t, reusedOp.index, int32(10))
}

func TestPollCacheFreeable(t *testing.T) {
	cache := &pollCache{}

	// Allocate operators
	op1 := cache.alloc()
	op2 := cache.alloc()

	require.NotNil(t, op1)
	require.NotNil(t, op2)

	// Mark operators as freeable
	cache.freeable(op1)
	cache.freeable(op2)

	// Verify they are in freelist
	cache.lock.Lock()
	assert.Len(t, cache.freelist, 2)
	assert.Contains(t, cache.freelist, op1.index)
	assert.Contains(t, cache.freelist, op2.index)
	cache.lock.Unlock()
}

func TestPollCacheFree(t *testing.T) {
	cache := &pollCache{}

	// Allocate and mark operators as freeable
	var ops []*fdOperator
	for i := 0; i < 5; i++ {
		op := cache.alloc()
		require.NotNil(t, op)
		ops = append(ops, op)
		cache.freeable(op)
	}

	// Verify they are in freelist
	cache.lock.Lock()
	freelistLen := len(cache.freelist)
	cache.lock.Unlock()
	assert.Equal(t, 5, freelistLen)

	// Set freeack flag
	cache.free()

	// Verify freeack is set
	assert.Equal(t, int32(1), atomic.LoadInt32(&cache.freeack))

	// Call freeable again to trigger cleanup
	cache.freeable(ops[0])

	// Verify freelist was cleared (should be 1 for the newly added operator)
	cache.lock.Lock()
	finalFreelistLen := len(cache.freelist)
	cache.lock.Unlock()
	assert.Equal(t, 1, finalFreelistLen) // Only the newly added operator
}

func TestPollCacheConcurrent(t *testing.T) {
	cache := &pollCache{}

	const numGoroutines = 10
	const numAllocations = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Concurrent allocations
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()

			var ops []*fdOperator
			for j := 0; j < numAllocations; j++ {
				op := cache.alloc()
				if op != nil {
					ops = append(ops, op)
				}

				// Randomly mark some as freeable
				if j%3 == 0 && len(ops) > 0 {
					freeableOp := ops[0]
					ops = ops[1:]
					cache.freeable(freeableOp)
				}
			}

			// Mark remaining as freeable
			for _, op := range ops {
				cache.freeable(op)
			}
		}()
	}

	wg.Wait()

	// Verify cache is still functional
	finalOp := cache.alloc()
	require.NotNil(t, finalOp)
}

func TestFDOperatorFields(t *testing.T) {
	op := &fdOperator{
		index: 42,
		fd:    123,
	}

	assert.Equal(t, int32(42), op.index)
	assert.Equal(t, int(123), op.fd)
	assert.Nil(t, op.link)
	assert.Nil(t, op.conn)
}

func TestFDOperatorSize(t *testing.T) {
	// Test that fdOperator has consistent size
	size1 := unsafe.Sizeof(fdOperator{})
	size2 := unsafe.Sizeof(fdOperator{})
	assert.Equal(t, size1, size2)

	// Should have reasonable size (not too large, not too small)
	assert.Greater(t, size1, uintptr(16)) // At least contains fields
	assert.Less(t, size1, uintptr(256))   // Not excessively large
}

func TestPollCacheBlockAllocation(t *testing.T) {
	cache := &pollCache{}

	// Calculate expected number of operators per block
	pdSize := unsafe.Sizeof(fdOperator{})
	expectedPerBlock := pollBlockSize / pdSize
	if expectedPerBlock == 0 {
		expectedPerBlock = 1
	}

	// Allocate more than one block worth
	var ops []*fdOperator
	allocations := int(expectedPerBlock) + 10

	for i := 0; i < allocations; i++ {
		op := cache.alloc()
		require.NotNil(t, op, "Allocation %d should succeed", i)
		ops = append(ops, op)
	}

	// Verify all have unique indices
	indices := make(map[int32]struct{})
	for _, op := range ops {
		_, exists := indices[op.index]
		assert.False(t, exists, "Index %d should be unique", op.index)
		indices[op.index] = struct{}{}
	}
}

func TestPollCacheFreeAckRace(t *testing.T) {
	cache := &pollCache{}

	const numOperations = 1000
	var freeCount int64

	// Start goroutine that marks operators as freeable
	go func() {
		for i := 0; i < numOperations; i++ {
			op := cache.alloc()
			if op != nil {
				cache.freeable(op)
				atomic.AddInt64(&freeCount, 1)
			}
			time.Sleep(time.Microsecond) // Small delay to increase race chance
		}
	}()

	// Start goroutine that calls free() periodically
	go func() {
		for i := 0; i < numOperations/10; i++ {
			cache.free()
			time.Sleep(10 * time.Microsecond)
		}
	}()

	time.Sleep(100 * time.Millisecond) // Let goroutines work

	// Verify no panics or corruption
	finalOp := cache.alloc()
	require.NotNil(t, finalOp)

	// Verify some operations completed
	assert.Greater(t, atomic.LoadInt64(&freeCount), int64(0))
}

func BenchmarkPollCacheAlloc(b *testing.B) {
	cache := &pollCache{}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			op := cache.alloc()
			if op != nil {
				// Simulate some usage
				op.fd = 42
				op.index = 1
			}
		}
	})
}

func BenchmarkPollCacheFreeable(b *testing.B) {
	cache := &pollCache{}

	// Pre-allocate some operators
	ops := make([]*fdOperator, 1000)
	for i := range ops {
		ops[i] = cache.alloc()
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			cache.freeable(ops[i%len(ops)])
			i++
		}
	})
}

func BenchmarkPollCacheAllocFreeCycle(b *testing.B) {
	cache := &pollCache{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		op := cache.alloc()
		if op != nil {
			cache.freeable(op)
			if i%100 == 0 {
				cache.free() // Trigger cleanup occasionally
			}
		}
	}
}
