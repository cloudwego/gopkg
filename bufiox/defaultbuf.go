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

import (
	"errors"
	"io"
	"net"

	"github.com/bytedance/gopkg/lang/mcache"
)

const maxConsecutiveEmptyReads = 100

var _ Reader = &DefaultReader{}

type DefaultReader struct {
	buf    []byte // buf[ri:] is the buffer for reading.
	ri     int    // buf read positions
	toFree [][]byte

	rn int // read len

	rd  io.Reader // reader provided by the client
	err error

	maxSizeStats maxSizeStats
}

const (
	defaultBufSize       = 8 * 1024
	nocopyWriteThreshold = 4 * 1024
)

var errNegativeCount = errors.New("bufiox: negative count")

// NewDefaultReader returns a new DefaultReader that reads from r.
func NewDefaultReader(rd io.Reader) *DefaultReader {
	r := &DefaultReader{rd: rd}
	return r
}

func (r *DefaultReader) acquireSlow(n int) int {
	if r.err != nil {
		return len(r.buf) - r.ri
	}

	if n > cap(r.buf)-r.ri {
		// new buffer
		ncap := (cap(r.buf) - r.ri) * 2
		if ncap < defaultBufSize {
			ncap = defaultBufSize
		}
		for ; ncap < n; ncap *= 2 {
		}
		r.toFree = append(r.toFree, r.buf)
		nbuf := mcache.Malloc(ncap)
		cn := copy(nbuf, r.buf[r.ri:])
		r.buf = nbuf[:cn]
		r.ri = 0
	}

	for i := 0; i < maxConsecutiveEmptyReads; i++ {
		m, err := r.rd.Read(r.buf[len(r.buf):cap(r.buf)])
		r.buf = r.buf[:len(r.buf)+m]
		if err != nil {
			r.err = err
			return len(r.buf) - r.ri
		}
		if n <= len(r.buf)-r.ri {
			return n
		}
	}
	return len(r.buf) - r.ri
}

// fill reads a new chunk into the buffer.
func (r *DefaultReader) acquire(n int) int {
	// fast path, for inline
	if n <= len(r.buf)-r.ri {
		return n
	}
	return r.acquireSlow(n)
}

func (r *DefaultReader) Next(n int) (buf []byte, err error) {
	if n < 0 {
		err = errNegativeCount
		return
	}
	m := r.acquire(n)
	if n > m {
		err = r.err
		return
	}
	// nocopy read
	buf = r.buf[r.ri : r.ri+n]
	r.ri += n
	r.rn += n
	return
}

func (r *DefaultReader) Peek(n int) (buf []byte, err error) {
	if n < 0 {
		err = errNegativeCount
		return
	}
	m := r.acquire(n)
	if n > m {
		err = r.err
		return
	}
	// nocopy read
	buf = r.buf[r.ri : r.ri+n]
	return
}

func (r *DefaultReader) Skip(n int) (err error) {
	if n < 0 {
		err = errNegativeCount
		return
	}
	m := r.acquire(n)
	if n > m {
		err = r.err
		return
	}
	r.ri += n
	r.rn += n
	return
}

func (r *DefaultReader) ReadLen() (n int) {
	return r.rn
}

func (r *DefaultReader) ReadBinary(bs []byte) (m int, err error) {
	m = r.acquire(len(bs))
	copy(bs, r.buf[r.ri:r.ri+m])
	r.ri += m
	r.rn += m
	if len(bs) > m {
		err = r.err
	}
	return
}

// Read implements io.Reader
// If some data is available but fewer than len(bs) bytes, Read returns what is available instead of waiting for more,
// which differs from ReadBinary.
func (r *DefaultReader) Read(bs []byte) (n int, err error) {
	if len(bs) == 0 {
		return 0, nil
	}
	if available := len(r.buf) - r.ri; available != 0 {
		// return the available data instead of waiting for more
		n = copy(bs, r.buf[r.ri:])
		r.ri += n
		r.rn += n
		return n, nil
	}
	// try to read
	m := r.acquire(1)
	if m < 1 {
		return 0, r.err
	}
	n = copy(bs, r.buf[r.ri:])
	r.ri += n
	r.rn += n
	return n, nil
}

func (r *DefaultReader) Release(e error) error {
	if r.toFree != nil {
		for i, buf := range r.toFree {
			mcache.Free(buf)
			r.toFree[i] = nil
		}
		r.toFree = r.toFree[:0]
	}
	if len(r.buf)-r.ri == 0 {
		// release buf
		r.maxSizeStats.update(r.rn)
		if cap(r.buf) > 0 {
			mcache.Free(r.buf)
		}
		r.buf = nil
		r.ri = 0
	}
	r.rn = 0
	return nil
}

var _ Writer = &DefaultWriter{}

type DefaultWriter struct {
	chunk  []byte
	chunks net.Buffers // [][]byte

	wl int // written len

	toFree [][]byte

	wd  io.Writer
	err error
}

// NewDefaultWriter returns a new DefaultWriter that writes to w.
func NewDefaultWriter(wd io.Writer) *DefaultWriter {
	w := &DefaultWriter{wd: wd}
	return w
}

func (w *DefaultWriter) acquire(n int) {
	// fast path, for inline
	if len(w.chunk)+n <= cap(w.chunk) {
		return
	}
	w.acquireSlow(n)
}

func (w *DefaultWriter) acquireSlow(n int) {
	if n > cap(w.chunk)-len(w.chunk) {
		if len(w.chunk) > 0 {
			w.chunks = append(w.chunks, w.chunk)
			w.chunk = nil
		}
		// new buffer
		var ncap int
		for ncap = defaultBufSize; ncap < n; ncap *= 2 {
		}
		w.chunk = mcache.Malloc(0, ncap)
		w.toFree = append(w.toFree, w.chunk)
	}
}

func (w *DefaultWriter) writeDirect(buf []byte) {
	if len(w.chunk) > 0 {
		w.chunks = append(w.chunks, w.chunk)
		w.chunk = nil
	}
	w.chunks = append(w.chunks, buf)
}

func (w *DefaultWriter) Malloc(n int) (buf []byte, err error) {
	if w.err != nil {
		err = w.err
		return
	}
	if n < 0 {
		err = errNegativeCount
		return
	}
	w.acquire(n)
	buf = w.chunk[len(w.chunk) : len(w.chunk)+n]
	w.chunk = w.chunk[:len(w.chunk)+n]

	w.wl += n
	return
}

func (w *DefaultWriter) WriteBinary(bs []byte) (n int, err error) {
	if w.err != nil {
		err = w.err
		return
	}
	if len(bs) >= nocopyWriteThreshold {
		w.writeDirect(bs)
		return len(bs), nil
	}
	w.acquire(len(bs))
	n = copy(w.chunk[len(w.chunk):cap(w.chunk)], bs)
	w.chunk = w.chunk[:len(w.chunk)+n]

	w.wl += len(bs)
	return
}

func (w *DefaultWriter) WrittenLen() int {
	return w.wl
}

func (w *DefaultWriter) Flush() (err error) {
	if w.err != nil {
		err = w.err
		return
	}
	if len(w.chunk) > 0 {
		w.chunks = append(w.chunks, w.chunk)
		w.chunk = nil
	}
	if len(w.chunks) == 0 {
		return nil
	}
	// might call writev if w.wd is net.Conn
	if _, err = w.chunks.WriteTo(w.wd); err != nil {
		w.err = err
		return err
	}
	w.chunk = nil
	for i := range w.chunks {
		w.chunks[i] = nil
	}
	w.chunks = w.chunks[:0]
	w.wl = 0
	if w.toFree != nil {
		for i, buf := range w.toFree {
			mcache.Free(buf)
			w.toFree[i] = nil
		}
		w.toFree = w.toFree[:0]
	}
	return nil
}

const statsBucketNum = 10

type maxSizeStats struct {
	buckets   [statsBucketNum]int
	bucketIdx int
}

func (s *maxSizeStats) update(size int) {
	s.buckets[s.bucketIdx] = size
	s.bucketIdx = (s.bucketIdx + 1) % statsBucketNum
}

func (s *maxSizeStats) maxSize() int {
	var maxSize int
	for _, size := range s.buckets {
		if maxSize < size {
			maxSize = size
		}
	}
	return maxSize
}
