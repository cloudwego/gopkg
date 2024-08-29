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

	"github.com/bytedance/gopkg/lang/dirtmake"
	"github.com/bytedance/gopkg/lang/mcache"
)

const maxConsecutiveEmptyReads = 100

var _ Reader = &DefaultReader{}

type DefaultReader struct {
	buf         []byte // buf[ri:] is the buffer for reading.
	bufReadOnly bool
	pendingBuf  [][]byte

	rd  io.Reader // reader provided by the client
	ri  int       // buf read positions
	err error

	maxSizeStats maxSizeStats
}

const (
	defaultBufSize = 4096
)

var errNegativeCount = errors.New("bufiox: negative count")

// NewDefaultReader returns a new DefaultReader that reads from r.
func NewDefaultReader(rd io.Reader) *DefaultReader {
	r := &DefaultReader{}
	r.reset(rd, nil)
	return r
}

// NewBytesReader returns a new DefaultReader that reads from buf[:len(buf)].
// Its operation on buf is read-only.
func NewBytesReader(buf []byte) *BytesReader {
	r := &BytesReader{}
	r.reset(r.fakedIOReader, buf)
	return r
}

type BytesReader struct {
	DefaultReader
	fakedIOReader fakeIOReader
}

func (r *DefaultReader) reset(rd io.Reader, buf []byte) {
	if cap(buf) > 0 {
		// set readOnly for buf from outside
		*r = DefaultReader{buf: buf, rd: rd, bufReadOnly: true}
	} else {
		*r = DefaultReader{buf: nil, rd: rd}
	}
}

func (r *DefaultReader) acquireSlow(n int) int {
	if r.err != nil {
		return len(r.buf) - r.ri
	}

	if cap(r.buf) == 0 {
		maxSize := r.maxSizeStats.maxSize()
		if maxSize < defaultBufSize {
			maxSize = defaultBufSize
		}
		for ; maxSize < n; maxSize *= 2 {
		}
		r.buf = mcache.Malloc(0, maxSize)
		r.bufReadOnly = false
	}

	if n > cap(r.buf)-r.ri {
		// grow buffer
		var ncap int
		for ncap = cap(r.buf) * 2; ncap-r.ri < n; ncap *= 2 {
		}
		nbuf := mcache.Malloc(ncap)
		if !r.bufReadOnly {
			r.pendingBuf = append(r.pendingBuf, r.buf)
		}
		cn := copy(nbuf[r.ri:], r.buf[r.ri:])
		r.buf = nbuf[:(r.ri + cn)]
		r.bufReadOnly = false
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
	return
}

func (r *DefaultReader) ReadLen() (n int) {
	return r.ri
}

func (r *DefaultReader) ReadBinary(bs []byte) (m int, err error) {
	m = r.acquire(len(bs))
	copy(bs, r.buf[r.ri:r.ri+m])
	r.ri += m
	if len(bs) > m {
		err = r.err
	}
	return
}

func (r *DefaultReader) Release(e error) error {
	if r.pendingBuf != nil {
		for _, buf := range r.pendingBuf {
			mcache.Free(buf)
		}
	}
	r.pendingBuf = nil
	if len(r.buf)-r.ri == 0 {
		// release buf
		r.maxSizeStats.update(cap(r.buf))
		if !r.bufReadOnly && cap(r.buf) > 0 {
			mcache.Free(r.buf)
		}
		r.buf = nil
	} else {
		if r.bufReadOnly {
			r.buf = r.buf[r.ri:]
		} else {
			n := copy(r.buf, r.buf[r.ri:])
			r.buf = r.buf[:n]
		}
	}
	r.ri = 0
	return nil
}

type fakeIOReader struct{}

func (fakeIOReader) Read(p []byte) (n int, err error) {
	return 0, io.EOF
}

var _ Writer = &DefaultWriter{}

type DefaultWriter struct {
	buf        []byte
	pendingBuf [][]byte

	wd  io.Writer
	err error

	maxSizeStats maxSizeStats

	disableCache bool
}

// NewDefaultWriter returns a new DefaultWriter that writes to w.
func NewDefaultWriter(wd io.Writer) *DefaultWriter {
	w := &DefaultWriter{}
	w.reset(wd, nil, false)
	return w
}

// NewBytesWriter returns a new DefaultWriter that writes to buf[len(buf):cap(buf)].
// The WrittenLen is set to len(buf) before the first write.
func NewBytesWriter(buf *[]byte) *BytesWriter {
	w := &BytesWriter{}
	w.fakedIOWriter.bw = w
	w.flushBytes = buf
	w.reset(&w.fakedIOWriter, *buf, true)
	return w
}

type BytesWriter struct {
	DefaultWriter
	fakedIOWriter fakeIOWriter
	flushBytes    *[]byte
}

func (w *DefaultWriter) reset(wd io.Writer, buf []byte, disableCache bool) {
	*w = DefaultWriter{buf: buf, wd: wd, disableCache: disableCache}
}

func (w *DefaultWriter) acquire(n int) {
	// fast path, for inline
	if len(w.buf)+n <= cap(w.buf) {
		return
	}
	w.acquireSlow(n)
}

func (w *DefaultWriter) acquireSlow(n int) {
	if cap(w.buf) == 0 {
		maxSize := w.maxSizeStats.maxSize()
		if maxSize < defaultBufSize {
			maxSize = defaultBufSize
		}
		for ; maxSize < n; maxSize *= 2 {
		}
		if w.disableCache {
			w.buf = dirtmake.Bytes(0, maxSize)
		} else {
			w.buf = mcache.Malloc(0, maxSize)
		}
	}

	if n > cap(w.buf)-len(w.buf) {
		// grow buffer
		var ncap int
		// reserve the length of len(w.buf) for copying data from the old buffer during flush
		for ncap = cap(w.buf) * 2; ncap-len(w.buf) < n; ncap *= 2 {
		}
		var nbuf []byte
		if w.disableCache {
			nbuf = dirtmake.Bytes(ncap, ncap)
		} else {
			nbuf = mcache.Malloc(ncap)
		}
		w.pendingBuf = append(w.pendingBuf, w.buf)
		// delay copying until flushing
		w.buf = nbuf[:len(w.buf)]
	}
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
	buf = w.buf[len(w.buf) : len(w.buf)+n]
	w.buf = w.buf[:len(w.buf)+n]
	return
}

func (w *DefaultWriter) WriteBinary(bs []byte) (n int, err error) {
	if w.err != nil {
		err = w.err
		return
	}
	w.acquire(len(bs))
	n = copy(w.buf[len(w.buf):cap(w.buf)], bs)
	w.buf = w.buf[:len(w.buf)+n]
	return
}

func (w *DefaultWriter) WrittenLen() int {
	return len(w.buf)
}

func (w *DefaultWriter) Flush() (err error) {
	if w.err != nil {
		err = w.err
		return
	}
	if w.buf == nil {
		return nil
	}
	// copy old buffer
	var offset int
	for _, oldBuf := range w.pendingBuf {
		offset += copy(w.buf[offset:], oldBuf[offset:])
	}
	if _, err = w.wd.Write(w.buf); err != nil {
		w.err = err
		return err
	}
	w.maxSizeStats.update(cap(w.buf))
	if !w.disableCache {
		if cap(w.buf) > 0 {
			mcache.Free(w.buf)
		}
		if w.pendingBuf != nil {
			for _, buf := range w.pendingBuf {
				mcache.Free(buf)
			}
		}
	}
	w.buf = nil
	w.pendingBuf = nil
	return nil
}

type fakeIOWriter struct {
	bw *BytesWriter
}

func (w *fakeIOWriter) Write(p []byte) (n int, err error) {
	*w.bw.flushBytes = p
	return len(p), nil
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
