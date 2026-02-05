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
	chunks [][]byte
	pool   [][]byte
}

func NewWriteBuffer() *WriteBuffer {
	return writeBufferPool.Get().(*WriteBuffer)
}

func (b *WriteBuffer) NewBuffer(old []byte, n int) []byte {
	if b == nil {
		return old
	}
	if len(old) > 0 {
		b.chunks = append(b.chunks, old)
	}
	if n < 0 {
		// n < 0 means no need to malloc
		return nil
	}
	// refresh chunk
	if n < padLength {
		n = padLength
	}
	buf := mcache.Malloc(n)
	buf = buf[:0]
	b.pool = append(b.pool, buf)
	return buf
}

func (b *WriteBuffer) Free() {
	if b == nil {
		return
	}
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

func (b *WriteBuffer) WriteDirect(old, buf []byte) []byte {
	if b == nil {
		return append(old, buf...)
	}
	// relink chunks
	if len(old) > 0 {
		b.chunks = append(b.chunks, old)
	}

	// write directly
	b.chunks = append(b.chunks, buf)

	if cap(buf)-len(buf) > 0 {
		return old[len(old):cap(old)]
	}
	return b.NewBuffer(nil, 0)
}

func (b *WriteBuffer) Bytes() [][]byte {
	if b == nil {
		return nil
	}
	return b.chunks
}
