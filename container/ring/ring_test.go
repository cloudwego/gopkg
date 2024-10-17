/*
 * Copyright 2024 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package ring

import (
	"container/ring"
	"fmt"
	"math/rand"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

type ringItem struct {
	value int
}

func newRandomValue(n int) []int {
	vs := make([]int, 0, n)
	for i := 0; i < n; i++ {
		vs = append(vs, rand.Intn(n))
	}
	return vs
}

func newRingItemSlice(vs []int) []ringItem {
	items := make([]ringItem, 0, len(vs))
	for i := 0; i < len(vs); i++ {
		items = append(items, ringItem{value: vs[i]})
	}
	return items
}

func newStdRing(vs []ringItem) *ring.Ring {
	r := ring.New(len(vs))
	for i := 0; i < len(vs); i++ {
		r.Value = &vs[i]
		r = r.Next()
	}
	return r
}

func TestRing(t *testing.T) {
	n := 100
	vs := newRandomValue(n)

	r := NewFromSlice(newRingItemSlice(vs))
	// Get
	for i := 0; i < n; i++ {
		it, ok := r.Get(i)
		assert.True(t, ok)
		assert.Equal(t, vs[i], it.Value().value)
		assert.Equal(t, vs[i], it.Pointer().value)
	}
	// Next
	curr := r.Head()
	h, _ := r.Get(0)
	assert.Equal(t, curr, h)
	for i := 0; i < n; i++ {
		next, ok := r.Next(curr.Index())
		assert.True(t, ok)
		curr = next
	}
	assert.Equal(t, curr, h) // back to head
	_, ok := r.Next(n + 1)
	assert.False(t, ok)
	// Prev
	for i := 0; i < n; i++ {
		prev, ok := r.Prev(curr.Index())
		assert.True(t, ok)
		curr = prev
	}
	assert.Equal(t, curr, h) // back to head
	_, ok = r.Prev(n + 1)
	assert.False(t, ok)
	// Do
	var (
		expectedTotal int
		actualTotal   int
	)
	r.Do(func(v *ringItem) {
		actualTotal += v.value
	})
	for i := 0; i < n; i++ {
		expectedTotal += vs[i]
	}
	assert.Equal(t, expectedTotal, actualTotal)
	// Modify
	for i := 0; i < n; i++ {
		it, ok := r.Get(i)
		assert.True(t, ok)
		newValue := i
		it.Pointer().value = newValue
		assert.Equal(t, newValue, it.Value().value)
	}
}

func TestMove(t *testing.T) {
	n := 100
	vs := newRandomValue(n)
	r := NewFromSlice(newRingItemSlice(vs))

	realNext, _ := r.Move(98, 2)
	expectedNext, _ := r.Get(0)
	assert.Equal(t, realNext, expectedNext)

	realNext, _ = r.Move(98, n+1)
	expectedNext, _ = r.Get(99)
	assert.Equal(t, realNext, expectedNext)

	realNext, _ = r.Move(1, -2)
	expectedNext, _ = r.Get(99)
	assert.Equal(t, realNext, expectedNext)

	realNext, _ = r.Move(1, -(2 + n))
	expectedNext, _ = r.Get(99)
	assert.Equal(t, realNext, expectedNext)
}

func BenchmarkNew(b *testing.B) {
	nn := []int{100000, 400000}
	for _, n := range nn {
		vs := newRandomValue(n)

		b.Run(fmt.Sprintf("std-keysize_n_%d", n), func(b *testing.B) {
			b.ResetTimer()
			for j := 0; j < b.N; j++ {
				stdRing := newStdRing(newRingItemSlice(vs))
				_ = stdRing
			}
		})
		runtime.GC()

		b.Run(fmt.Sprintf("new-keysize_n_%d", n), func(b *testing.B) {
			b.ResetTimer()
			for j := 0; j < b.N; j++ {
				newRing := NewFromSlice(newRingItemSlice(vs))
				_ = newRing
			}
		})
		runtime.GC()
	}
}

func BenchmarkDo(b *testing.B) {
	nn := []int{10000, 40000}
	for _, n := range nn {
		vs := newRandomValue(n)
		b.Run(fmt.Sprintf("std-keysize_n_%d", n), func(b *testing.B) {
			b.ResetTimer()
			stdRing := newStdRing(newRingItemSlice(vs))
			for j := 0; j < b.N; j++ {
				stdRing.Do(func(i any) {})
			}
		})
		runtime.GC()

		b.Run(fmt.Sprintf("new-keysize_n_%d", n), func(b *testing.B) {
			b.ResetTimer()
			newRing := NewFromSlice(newRingItemSlice(vs))
			for j := 0; j < b.N; j++ {
				newRing.Do(func(i *ringItem) {})
			}
		})
		runtime.GC()
	}
}

func BenchmarkGC(b *testing.B) {
	nn := []int{100000, 400000}
	for _, n := range nn {
		vs := newRandomValue(n)

		b.Run(fmt.Sprintf("std-keysize_n_%d", n), func(b *testing.B) {
			stdRing := newStdRing(newRingItemSlice(vs))
			b.ResetTimer()
			for j := 0; j < b.N; j++ {
				runtime.GC()
			}
			runtime.KeepAlive(stdRing)
			stdRing = nil
			_ = stdRing
		})
		runtime.GC()

		b.Run(fmt.Sprintf("new-keysize_n_%d", n), func(b *testing.B) {
			newRing := NewFromSlice(newRingItemSlice(vs))
			b.ResetTimer()
			for j := 0; j < b.N; j++ {
				runtime.GC()
			}
			runtime.KeepAlive(newRing)
			newRing = nil
			_ = newRing
		})
		runtime.GC()
	}
}
