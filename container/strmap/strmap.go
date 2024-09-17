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

package strmap

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/cloudwego/gopkg/internal/hack"
	"github.com/cloudwego/gopkg/internal/hash/maphash"
	"github.com/cloudwego/gopkg/internal/strstore"
)

// StrMap represents GC friendly readonly string map implementation.
// type V must NOT contain pointer for performance concern.
type StrMap[V any] struct {
	// `data` holds bytes of keys
	data []byte

	// `items` holds key meta
	items []mapItem[V]

	// max hashtable ~ 2 billions which means len(items) < the num as well.
	hashtable []int32 // using int32 for mem efficiency

	// for maphash
	seed maphash.Seed
}

type mapItem[V any] struct {
	off  int
	sz   uint32 // 4GB, big enough for key
	slot uint32
	v    V
}

// New creates a StrMap instance,
func New[V any]() *StrMap[V] {
	return &StrMap[V]{seed: maphash.MakeSeed()}
}

// NewFromMap creates StrMap from map
func NewFromMap[V any](m map[string]V) *StrMap[V] {
	ret := New[V]()
	if err := ret.LoadFromMap(m); err != nil {
		panic(err)
	}
	return ret
}

// NewFromSlice creates StrMap from slices, len(kk) must equal to len(vv)
func NewFromSlice[V any](kk []string, vv []V) *StrMap[V] {
	ret := New[V]()
	if err := ret.LoadFromSlice(kk, vv); err != nil {
		panic(err)
	}
	return ret
}

// LoadFromMap resets StrMap and loads from map
func (p *StrMap[V]) LoadFromMap(m map[string]V) error {
	kk := make([]string, 0, len(m))
	vv := make([]V, 0, len(m))
	for k, v := range m {
		kk = append(kk, k)
		vv = append(vv, v)
	}
	return p.LoadFromSlice(kk, vv)
}

// LoadFromSlice resets StrMap and loads from slices, len(kk) must equal to len(vv)
func (m *StrMap[V]) LoadFromSlice(kk []string, vv []V) error {
	if len(kk) != len(vv) {
		return errors.New("kv len not match")
	}
	m.data = m.data[:0]
	m.items = m.items[:0]
	m.hashtable = m.hashtable[:0]

	sz := 0
	for _, k := range kk {
		sz += len(k)
	}
	if cap(m.data) < sz {
		m.data = make([]byte, 0, sz)
	}
	if cap(m.items) < len(vv) {
		m.items = make([]mapItem[V], 0, len(vv))
	}

	for i, k := range kk {
		if len(k) > math.MaxUint32 {
			// it doesn't make sense ...
			return errors.New("key too large")
		}
		v := vv[i]
		m.items = append(m.items,
			mapItem[V]{
				off:  len(m.data),
				sz:   uint32(len(k)),
				slot: uint32(maphash.String(m.seed, k)),
				v:    v,
			})
		m.data = append(m.data, k...)
	}
	m.makeHashtable()
	return nil
}

// Len returns the size of map
func (m *StrMap[V]) Len() int {
	return len(m.items)
}

// Item returns the i'th item in map.
// It panics if i is not in the range [0, Len()).
func (m *StrMap[V]) Item(i int) (string, V) {
	e := &m.items[i]
	return hack.ByteSliceToString(m.data[e.off : e.off+int(e.sz)]), e.v
}

type itemsBySlot[V any] []mapItem[V]

func (x itemsBySlot[V]) Len() int           { return len(x) }
func (x itemsBySlot[V]) Less(i, j int) bool { return x[i].slot < x[j].slot }
func (x itemsBySlot[V]) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

func (m *StrMap[V]) makeHashtable() {
	slots := calcHashtableSlots(len(m.items))
	if cap(m.hashtable) < int(slots) {
		m.hashtable = make([]int32, slots)
	} else {
		m.hashtable = m.hashtable[:slots]
	}

	// update `slot` of mapItem to fit the size of hashtable
	for i := range m.items {
		m.items[i].slot = m.items[i].slot % uint32(slots)
	}

	// make sure items with the same slot stored together
	// good for cpu cache
	sort.Sort(itemsBySlot[V](m.items))

	for i := 0; i < len(m.hashtable); i++ {
		m.hashtable[i] = -1
	}
	for i := range m.items {
		e := &m.items[i]
		if m.hashtable[e.slot] < 0 {
			// we only need to store the 1st item if hash conflict
			// since they're already stored together
			// will check the next item when Get
			m.hashtable[e.slot] = int32(i)
		}
	}
}

// Get ...
func (m *StrMap[V]) Get(s string) (t V, ok bool) {
	slot := uint32(maphash.String(m.seed, s)) % uint32(len(m.hashtable))
	i := m.hashtable[slot]
	if i < 0 {
		return t, false
	}
	e := &m.items[i]
	if string(m.data[e.off:e.off+int(e.sz)]) == s {
		return e.v, true
	}

	// collision, worst O(n)
	// coz i always point to the 1st item with the same slot,
	// can scan till m.items ends or e.slot != slot.
	for j := i + 1; j < int32(len(m.items)); j++ {
		e = &m.items[j]
		if e.slot != slot {
			break
		}
		if string(m.data[e.off:e.off+int(e.sz)]) == s {
			return e.v, true
		}
	}
	return t, false
}

// String ...
func (m *StrMap[V]) String() string {
	b := &strings.Builder{}
	b.WriteString("{\n")
	for _, e := range m.items {
		fmt.Fprintf(b, "%q: %v,\n", string(m.data[e.off:e.off+int(e.sz)]), e.v)
	}
	b.WriteString("}")
	return b.String()
}

func (m *StrMap[V]) debugString() string {
	b := &strings.Builder{}
	b.WriteString("{\n")
	for _, e := range m.items {
		fmt.Fprintf(b, "{off:%d, slot:%x, str:%q, v:%v},\n", e.off, e.slot, string(m.data[e.off:e.off+int(e.sz)]), e.v)
	}
	fmt.Fprintf(b, "}(slots=%d, items=%d)", len(m.hashtable), len(m.items))
	return b.String()
}

// Str2Str uses StrMap and strstore.StrStore to store map[string]string
type Str2Str struct {
	strMap   *StrMap[int]
	strStore *strstore.StrStore
}

func NewStr2Str() *Str2Str {
	return &Str2Str{}
}

// NewStr2StrFromSlice creates StrMapStr2Str from key, value slices.
func NewStr2StrFromSlice(kk, vv []string) *Str2Str {
	m := NewStr2Str()
	m.LoadFromSlice(kk, vv)
	return m
}

// NewStr2StrFromMap creates StrMapStr2Str from map.
func NewStr2StrFromMap(m map[string]string) *Str2Str {
	sm := NewStr2Str()
	sm.LoadFromMap(m)
	return sm
}

// LoadFromSlice resets Str2Str and loads from slices.
func (sm *Str2Str) LoadFromSlice(kk, vv []string) {
	ss, ids := strstore.New(vv)
	sm.strStore = ss
	sm.strMap = NewFromSlice(kk, ids)
}

// LoadFromMap resets Str2Str and loads from map.
func (sm *Str2Str) LoadFromMap(m map[string]string) {
	kk := make([]string, 0, len(m))
	vv := make([]string, 0, len(m))
	for k, v := range m {
		kk = append(kk, k)
		vv = append(vv, v)
	}
	sm.LoadFromSlice(kk, vv)
}

// Get ...
func (sm *Str2Str) Get(k string) (string, bool) {
	if idx, ok := sm.strMap.Get(k); ok {
		v := sm.strStore.Get(idx)
		// TODO: any check?
		return v, true
	}
	return "", false
}

// Len returns the size of map
func (sm *Str2Str) Len() int {
	return sm.strMap.Len()
}
