// Copyright 2024 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package strstore

import (
	"math"
	"unsafe"
)

const (
	strlenSize = 4 // size of uint32, maximum 4GB for each string
)

// StrStore is used to store string with less GC overhead.
// The string stored here should not be longer than `pageSize` and does not need to be deleted.
type StrStore struct {
	buf []byte
}

// New creates a StrStore instance.
func New() *StrStore {
	return &StrStore{}
}

// NewFromSlice constructs a StrStore with the input string slice and returns the StrStore and indexes for the following reads.
// It panics if any string in the slice is longer than math.MaxUint32.
func NewFromSlice(ss []string) (*StrStore, []int) {
	st := &StrStore{}
	idxes, err := st.Load(ss)
	if err != nil {
		panic(err)
	}
	return st, idxes
}

// Load resets the StrStore and set from input string slices.
func (s *StrStore) Load(ss []string) ([]int, error) {
	n := len(ss)
	totalLen := strlenSize * n
	for i := 0; i < n; i++ {
		if len(ss[i]) > math.MaxUint32 {
			panic("string too long")
		}
		totalLen += len(ss[i])
	}
	idxes := make([]int, n)
	if cap(s.buf) < totalLen {
		s.buf = make([]byte, totalLen)
	} else {
		s.buf = s.buf[:totalLen]
	}

	offset := 0
	for i := 0; i < n; i++ {
		idxes[i] = offset
		*(*uint32)(unsafe.Pointer(&s.buf[offset])) = uint32(len(ss[i]))
		copy(s.buf[offset+strlenSize:offset+strlenSize+len(ss[i])], ss[i])
		offset += strlenSize + len(ss[i])
	}
	return idxes, nil
}

// Get gets the string with the idx.
// It returns empty string if the no string can be found with the input idx
func (s *StrStore) Get(idx int) string {
	if idx < 0 || idx >= len(s.buf) {
		return ""
	}
	length := *(*uint32)(unsafe.Pointer(&s.buf[idx]))
	b := s.buf[idx+strlenSize : idx+strlenSize+int(length)]
	return *(*string)(unsafe.Pointer(&b))
}

// Len returns the total length of bytes.
func (s *StrStore) Len() int {
	return len(s.buf)
}
