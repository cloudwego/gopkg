/*
 * Copyright 2025 CloudWeGo Authors
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

package binding

import (
	"unsafe"
)

type sliceHeader struct {
	Data unsafe.Pointer
	Len  int
	Cap  int
}

// b2s converts byte slice to a string without memory allocation.
// The returned string shares memory with the input byte slice.
// WARNING: The string becomes invalid if the byte slice is modified or garbage collected.
// Only use when the byte slice lifecycle is well understood and controlled.
func b2s(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// s2b converts string to a byte slice without memory allocation.
// The returned slice shares memory with the input string.
// WARNING: Modifying the returned slice will cause undefined behavior since
// strings are immutable in Go. This should only be used for read-only operations.
func s2b(s string) (b []byte) {
	*(*string)(unsafe.Pointer(&b)) = s
	(*sliceHeader)(unsafe.Pointer(&b)).Cap = len(s)
	return
}
