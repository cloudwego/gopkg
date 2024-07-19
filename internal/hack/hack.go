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

package hack

import "unsafe"

type sliceHeader struct {
	Data uintptr
	Len  int
	Cap  int
}

type strHeader struct {
	Data uintptr
	Len  int
}

// ByteSliceToString converts []byte to string without copy
func ByteSliceToString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// StringToByteSlice converts string to []byte without copy
func StringToByteSlice(s string) []byte {
	var v []byte
	p0 := (*sliceHeader)(unsafe.Pointer(&v))
	p1 := (*strHeader)(unsafe.Pointer(&s))
	p0.Data = p1.Data
	p0.Len = p1.Len
	p0.Cap = p1.Len
	return v
}
