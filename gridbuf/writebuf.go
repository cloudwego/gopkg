// Copyright 2025 CloudWeGo Authors
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

package gridbuf

import (
	"sync"

	"github.com/bytedance/gopkg/lang/mcache"
)

const padLength = 1 << 13

var writeBufferPool = sync.Pool{
	New: func() interface{} {
		return &WriteBuffer{
			chunks: make([][]byte, 0, 16),
			pool:   make([][]byte, 0, 16),
		}
	},
}

type WriteBuffer struct {
	off    int // write offset of chunk
	chunk  []byte
	chunks [][]byte
	pool   [][]byte
}

func NewWriteBuffer() *WriteBuffer {
	return writeBufferPool.Get().(*WriteBuffer)
}

func (b *WriteBuffer) Bytes() [][]byte {
	if b.off > 0 {
		b.chunks = append(b.chunks, b.chunk[:b.off])
		b.chunk = b.chunk[b.off:]
		b.off = 0
	}
	return b.chunks
}

func (b *WriteBuffer) Free() {
	b.off = 0
	b.chunk = nil
	for i := range b.chunks {
		b.chunks[i] = nil
	}
	b.chunks = b.chunks[:0]
	for i := range b.pool {
		mcache.Free(b.pool[i])
		b.pool[i] = nil
	}
	b.pool = b.pool[:0]
	writeBufferPool.Put(b)
}

// MallocN malloc n bytes from chunk, if chunk is not enough, it will grow.
//
// MAKE SURE IT CAN BE INLINE:
// `can inline (*XWriteBuffer).MallocN with cost 79`
func (b *WriteBuffer) MallocN(n int) (buf []byte) {
	buf = b.chunk[b.off:]
	if len(buf) < n {
		buf = b.growSlow(n)
	}
	b.off += n
	return
}

func (b *WriteBuffer) growSlow(n int) []byte {
	if b.off > 0 {
		b.chunk = b.chunk[:b.off]
		b.chunks = append(b.chunks, b.chunk)
		b.off = 0
	}
	// refresh chunk
	if n < padLength {
		n = padLength
	}
	buf := mcache.Malloc(n)
	buf = buf[:cap(buf)]
	b.pool = append(b.pool, buf)
	b.chunk = buf
	return buf
}

func (b *WriteBuffer) WriteDirect(buf []byte) {
	// relink chunks
	if b.off > 0 {
		b.chunks = append(b.chunks, b.chunk[:b.off])
		b.chunk = b.chunk[b.off:]
		b.off = 0
	}

	// write directly
	b.chunks = append(b.chunks, buf)
	return
}
