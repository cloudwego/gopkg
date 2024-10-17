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

// Ring is a GC friendly ring implementation.
// items are allocated by one malloc and cannot be resized. Item inside can be accesses and modified.
// type V must NOT contain pointer for performance concern.
type Ring[V any] struct {
	items []Item[V]
}

// Item is the element stored in the Ring
type Item[V any] struct {
	value V
	idx   int
}

func NewFromSlice[V any](vv []V) *Ring[V] {
	r := &Ring[V]{}
	r.items = make([]Item[V], len(vv))
	for i := 0; i < len(vv); i++ {
		r.items[i].value = vv[i]
		r.items[i].idx = i
	}
	return r
}

// Head returns the first item.
func (r *Ring[V]) Head() *Item[V] {
	if len(r.items) == 0 {
		return nil
	}
	return &r.items[0]
}

// Get returns the ith item.
func (r *Ring[V]) Get(i int) (*Item[V], bool) {
	if i < 0 || i >= len(r.items) {
		return nil, false
	}
	return &r.items[i], true
}

// Next returns the next item of the ith item.
// Return the first(idx=0) item if i == len(r.items) - 1.
func (r *Ring[V]) Next(i int) (*Item[V], bool) {
	if i < 0 || i >= len(r.items) {
		return nil, false
	}
	if i == len(r.items)-1 {
		return &r.items[0], true
	}
	return &r.items[i+1], true
}

// Prev returns the previous item of the ith item
// Return the last item(idx=len(items)-1) if i == 0.
func (r *Ring[V]) Prev(i int) (*Item[V], bool) {
	if i < 0 || i >= len(r.items) {
		return nil, false
	}
	if i == 0 {
		return &r.items[len(r.items)-1], true
	}
	return &r.items[i-1], true
}

// Move returns the item moving n step from the ith item.
func (r *Ring[V]) Move(i, n int) (*Item[V], bool) {
	if i < 0 || i >= len(r.items) {
		return nil, false
	}
	var idx int
	if n >= 0 {
		idx = (i + n) % len(r.items)
	} else {
		idx = len(r.items) + (i+n)%len(r.items)
	}
	return &r.items[idx], true
}

// Do calls function f on each item of the ring in forward order.
func (r *Ring[V]) Do(f func(v *V)) {
	for i := 0; i < len(r.items); i++ {
		f(&r.items[i].value)
	}
}

// Len returns the length of the ring.
func (r *Ring[V]) Len() int {
	return len(r.items)
}

// Index returns the index of the item in the ring.
func (it *Item[V]) Index() int {
	return it.idx
}

// Value returns the value of the item.
func (it *Item[V]) Value() V {
	return it.value
}

// Pointer returns the pointer of the item.
// Use Pointer if you want to modify V.
// Do not reference to the pointer from other place.
func (it *Item[V]) Pointer() *V {
	return &it.value
}
