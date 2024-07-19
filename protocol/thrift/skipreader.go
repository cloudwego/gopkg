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

package thrift

import (
	"io"
	"sync"
)

// this file contains readers for SkipDecoder

type skipReaderIface interface {
	Next(n int) (buf []byte, err error)
	Bytes() (buf []byte, err error)
	Release()
}

var poolSkipReader = sync.Pool{
	New: func() interface{} {
		return &skipReader{b: make([]byte, 1024)}
	},
}

var poolSkipRemoteBuffer = sync.Pool{
	New: func() interface{} {
		return &skipByteBuffer{}
	},
}

// skipReader ... general skip reader for io.Reader
type skipReader struct {
	r io.Reader

	p int
	b []byte
}

func newSkipReader(r io.Reader) *skipReader {
	ret := poolSkipReader.Get().(*skipReader)
	ret.Reset(r)
	return ret
}

func (p *skipReader) Release() {
	poolSkipReader.Put(p)
}

func (p *skipReader) Reset(r io.Reader) {
	p.r = r
	p.p = 0
}

func (p *skipReader) Bytes() ([]byte, error) {
	ret := p.b[:p.p]
	p.p = 0
	return ret, nil
}

func (p *skipReader) grow(n int) {
	// assert: len(p.b)-p.p < n
	sz := 2 * cap(p.b)
	if sz < p.p+n {
		sz = p.p + n
	}
	b := make([]byte, sz)
	copy(b, p.b[:p.p])
	p.b = b
}

func (p *skipReader) Next(n int) (buf []byte, err error) {
	if len(p.b)-p.p < n {
		p.grow(n)
	}
	if _, err := io.ReadFull(p.r, p.b[p.p:p.p+n]); err != nil {
		return nil, NewProtocolExceptionWithErr(err)
	}
	ret := p.b[p.p : p.p+n]
	p.p += n
	return ret, nil
}

// remoteByteBuffer ... github.com/cloudwego/kitex/pkg/remote.ByteBuffer
type remoteByteBuffer interface {
	Peek(n int) (buf []byte, err error)
	ReadableLen() (n int)
	Skip(n int) (err error)
}

// skipByteBuffer ... optimized zero copy skipreader for remote.ByteBuffer
type skipByteBuffer struct {
	p remoteByteBuffer

	r int
	b []byte
}

func newSkipByteBuffer(buf remoteByteBuffer) *skipByteBuffer {
	ret := poolSkipRemoteBuffer.Get().(*skipByteBuffer)
	ret.Reset(buf)
	return ret
}

func (p *skipByteBuffer) Release() {
	poolSkipRemoteBuffer.Put(p)
}

func (p *skipByteBuffer) Reset(buf remoteByteBuffer) {
	p.r = 0
	p.b = nil
	p.p = buf
}

func (p *skipByteBuffer) Bytes() ([]byte, error) {
	ret := p.b[:p.r]
	if err := p.p.Skip(p.r); err != nil {
		return nil, err
	}
	p.r = 0
	return ret, nil
}

// Next ...
func (p *skipByteBuffer) Next(n int) (ret []byte, err error) {
	if p.r+n < len(p.b) { // fast path
		ret, p.r = p.b[p.r:p.r+n], p.r+n
		return
	}
	return p.nextSlow(n)
}

func (p *skipByteBuffer) nextSlow(n int) ([]byte, error) {
	// trigger underlying conn to read more
	_, err := p.p.Peek(p.r + n)
	if err != nil {
		return nil, err
	}
	// read as much as possible, luckily, we will have a full buffer
	// then we no need to call p.Peek many times
	p.b, err = p.p.Peek(p.p.ReadableLen())
	if err != nil {
		return nil, err
	}
	// after calling p.p.Peek, p.buf MUST be at least (p.r + n) len
	ret := p.b[p.r : p.r+n]
	p.r += n
	return ret, nil
}
