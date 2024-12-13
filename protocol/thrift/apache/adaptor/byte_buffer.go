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

package adaptor

import "io"

// byteBuffer
type byteBuffer interface {
	// Next reads the next n bytes sequentially and returns the original buffer.
	Next(n int) (p []byte, err error)

	// ReadableLen returns the total length of readable buffer.
	// Return: -1 means unreadable.
	ReadableLen() (n int)

	// Malloc n bytes sequentially in the writer buffer.
	Malloc(n int) (buf []byte, err error)
}

// byteBufferWrapper is an adaptor that implement Read() by Next() and ReadableLen() and implement Write() by Malloc()
type byteBufferWrapper struct {
	b byteBuffer
}

func byteBuffer2ReadWriter(n byteBuffer) io.ReadWriter {
	return &byteBufferWrapper{b: n}
}

// Read reads data from the byteBufferWrapper's internal buffer into p.
func (bw byteBufferWrapper) Read(p []byte) (n int, err error) {
	readable := bw.b.ReadableLen()
	if readable == -1 {
		return 0, err
	}
	if readable > len(p) {
		readable = len(p)
	}
	data, err := bw.b.Next(readable)
	if err != nil {
		return -1, err
	}
	copy(p, data)
	return readable, nil
}

// Write writes data from the byteBufferWrapper's internal buffer into p.
func (bw byteBufferWrapper) Write(p []byte) (n int, err error) {
	data, err := bw.b.Malloc(len(p))
	if err != nil {
		return -1, err
	}
	copy(data, p)
	return len(data), nil
}
