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

package bufiox

// Writer is a buffer IO interface, which provides a user-space zero-copy method to reduce memory allocation and copy overhead.
type Writer interface {
	// Malloc returns a shallow copy of the write buffer with length n,
	// otherwise returns an error if it's unable to get n bytes from the write buffer.
	// Must ensure that the data written by the user to buf can be flushed to the underlying io.Writer.
	//
	// Caller cannot write data to the returned buf after calling Flush.
	Malloc(n int) (buf []byte, err error)

	// WriteBinary writes bs to the buffer, it may be a zero copy write.
	// MUST ensure that bs is not being concurrently written before calling Flush.
	// It returns err if n < len(bs), while n is the number of bytes written.
	WriteBinary(bs []byte) (n int, err error)

	// WrittenLen returns the total length of the buffer written.
	// Malloc / WriteBinary will increase the length. When the Flush function is called, WrittenLen is set to 0.
	WrittenLen() (length int)

	// Flush writes any malloc data to the underlying io.Writer, and reset WrittenLen to zero.
	Flush() (err error)
}
