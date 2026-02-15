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
	"errors"
	"sync"

	"github.com/bytedance/gopkg/lang/mcache"
)

var (
	errReadBufferNotEnough = errors.New("error grid read buffer not enough")
	readBufferPool         = sync.Pool{
		New: func() interface{} {
			return &ReadBuffer{
				pool: make([][]byte, 0, 16),
			}
		},
	}
)

type ReadBuffer struct {
	off    int
	chunk  []byte
	chunks [][]byte
	pool   [][]byte
}

func NewReadBuffer(bufs [][]byte) *ReadBuffer {
	rb := readBufferPool.Get().(*ReadBuffer)
	rb.chunk = bufs[0]
	rb.chunks = bufs[1:]
	return rb
}

// ReadN read n bytes from chunk, if chunk is not enough, it will read from next chunks.
//
// MAKE SURE IT CAN BE INLINE:
// `can inline (*XReadBuffer).ReadN with cost 80`
func (b *ReadBuffer) ReadN(n int) (buf []byte) {
	buf = b.chunk[b.off:]
	if len(buf) < n {
		buf = b.readSlow(n)
	} else {
		b.off += n
	}
	return
}

func (b *ReadBuffer) readSlow(n int) (buf []byte) {
	buf = mcache.Malloc(n)
	b.pool = append(b.pool, buf)
	var l, m int
	if len(b.chunk)-b.off > 0 {
		m = copy(buf[l:], b.chunk[b.off:])
		l += m
	}
	for l < n {
		if len(b.chunks) == 0 {
			panic(errReadBufferNotEnough.Error())
		}
		b.chunk = b.chunks[0]
		b.off = 0
		b.chunks = b.chunks[1:]
		m = copy(buf[l:], b.chunk)
		l += m
	}
	b.off += m
	return
}

// CopyBytes copy bytes from chunk, if chunk is not enough, it will copy from next chunks.
//
// MAKE SURE IT CAN BE INLINE:
// `can inline (*XReadBuffer).CopyBytes with cost 80`
func (b *ReadBuffer) CopyBytes(buf []byte) {
	n := copy(buf, b.chunk[b.off:])
	if len(buf) > n {
		b.copySlow(buf)
	} else {
		b.off += n
	}
}

func (b *ReadBuffer) copySlow(buf []byte) {
	m := len(b.chunk) - b.off
	l := m
	for l < len(buf) {
		if len(b.chunks) == 0 {
			panic(errReadBufferNotEnough.Error())
		}
		b.chunk = b.chunks[0]
		b.off = 0
		b.chunks = b.chunks[1:]
		m = copy(buf[l:], b.chunk)
		l += m
	}
	b.off += m
}

func (b *ReadBuffer) Free() {
	b.off = 0
	b.chunk = nil
	b.chunks = nil
	for i := range b.pool {
		mcache.Free(b.pool[i])
		b.pool[i] = nil
	}
	b.pool = b.pool[:0]
	readBufferPool.Put(b)
}
