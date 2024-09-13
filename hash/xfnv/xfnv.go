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

// Package xfnv is modified and non-cross-platform compatible version of FNV-1a.
//
// It computes 8 bytes per round by converting bytes to uint64 directly
// as a result it doesn't generate the same result for diff cpu arch.
package xfnv

import (
	"unsafe"
)

const (
	fnvHashOffset64 = uint64(14695981039346656037) // fnv hash offset64
	fnvHashPrime64  = uint64(1099511628211)
)

func strDataPtr(s string) unsafe.Pointer {
	// for str, the Data ptr is always the 1st field
	return *(*unsafe.Pointer)(unsafe.Pointer(&s))
}

func bytesDataPtr(b []byte) unsafe.Pointer {
	// for []byte, the Data ptr is always the 1st field
	return *(*unsafe.Pointer)(unsafe.Pointer(&b))
}

// Hash returns the hash of the given bytes
//
// DO NOT STORE the return value since it's NOT cross-platform compatible.
// It's designed for in-memory use.
func Hash(b []byte) uint64 {
	return doHash(bytesDataPtr(b), len(b))
}

// HashStr returns the hash of the given string
//
// DO NOT STORE the return value since it's NOT cross-platform compatible.
// It's designed for in-memory use.
func HashStr(s string) uint64 {
	return doHash(strDataPtr(s), len(s))
}

func doHash(p unsafe.Pointer, n int) uint64 {
	h := fnvHashOffset64
	i := 0
	// 8 byte per round
	for m := n >> 3; i < m; i++ {
		h ^= *(*uint64)(unsafe.Add(p, i<<3)) // p[i*8]
		h *= fnvHashPrime64
	}
	// left 0-7 bytes
	i = i << 3
	for ; i < n; i++ {
		h ^= uint64(*(*byte)(unsafe.Add(p, i)))
		h *= fnvHashPrime64
	}
	return h
}
