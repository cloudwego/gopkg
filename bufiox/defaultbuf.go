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
	defaultBufSize        = 8 * 1024
	nocopyWriteThreshold  = 4 * 1024
	directlyReadThreshold = 4 * 1024
)

var errNegativeCount = errors.New("bufiox: negative count")

// NewDefaultReader returns a new DefaultReader that reads from r.
func NewDefaultReader(rd io.Reader) *DefaultReader {
	r := &DefaultReader{rd: rd}
	return r
}

// read data to buf[len:cap] until len(buf) >= expectedLen, returns the new buffer length and any err encountered.
func (r *DefaultReader) readExpected(buf []byte, expectedLen int) (int, error) {
	for i := 0; i < maxConsecutiveEmptyReads; i++ {
		m, err := r.rd.Read(buf[len(buf):cap(buf)])
		buf = buf[:len(buf)+m]
		if err != nil {
			return len(buf), err
		}
		if expectedLen <= len(buf) {
			return len(buf), nil
		}
	}
	return len(buf), io.ErrNoProgress
}

// fill reads a new chunk into the buffer.
func (r *DefaultReader) acquire(n int, skip bool) int {
	if r.err != nil {
		return len(r.buf) - r.ri
	}

	if n > cap(r.buf)-r.ri {
		// calculate new size
		size := r.maxSizeStats.maxSize()
		if size < defaultBufSize {
			size = defaultBufSize
		}
		for ; size < n; size *= 2 {
		}
		buf := mcache.Malloc(size)
		if !skip {
			// copy remaining data
			copy(buf, r.buf[r.ri:])
		}
		// free stale buf
		r.toFree = append(r.toFree, r.buf)
		// set new buf
		r.buf = buf[:len(r.buf)-r.ri]
		r.ri = 0
	}

	var nl int
	nl, r.err = r.readExpected(r.buf[r.ri:], n)
	r.buf = r.buf[:r.ri+nl]
	return nl
}

func (r *DefaultReader) Next(n int) (buf []byte, err error) {
	if n < 0 {
		err = errNegativeCount
		return
	}
	if n > len(r.buf)-r.ri {
		m := r.acquire(n, false)
		if n > m {
			err = r.err
			return
		}
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
	if n > len(r.buf)-r.ri {
		m := r.acquire(n, false)
		if n > m {
			err = r.err
			return
		}
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
	if n > len(r.buf)-r.ri {
		m := r.acquire(n, true)
		if n > m {
			err = r.err
			return
		}
	}
	r.ri += n
	r.rn += n
	return
}

func (r *DefaultReader) ReadLen() (n int) {
	return r.rn
}

// ReadBinary reads exactly len(bs) bytes to bs, wait for reading from the underlying reader until done,
// or returns the actual read data length and err if there's no enough data.
func (r *DefaultReader) ReadBinary(bs []byte) (n int, err error) {
	if len(bs) == 0 {
		return
	}
	n = copy(bs, r.buf[r.ri:])
	r.ri += n
	if len(bs) > n {
		if len(bs)-n >= directlyReadThreshold {
			// If the data outside the buffer is greater than the threshold,
			// directly call Read to reducing copying overhead.
			n, r.err = r.readExpected(bs[:n:len(bs)], len(bs))
		} else {
			r.acquire(len(bs)-n, false)
			m := copy(bs[n:], r.buf[r.ri:])
			r.ri += m
			n = n + m
		}
		r.rn += n
		err = r.err
		return
	}
	r.rn += n
	return
}

// Read implements io.Reader
// If some data is available but fewer than len(bs) bytes, Read returns what is available instead of waiting for more,
// which differs from ReadBinary.
func (r *DefaultReader) Read(bs []byte) (n int, err error) {
	if len(bs) == 0 {
		return
	}
	n = copy(bs, r.buf[r.ri:])
	if n > 0 {
		r.ri += n
		r.rn += n
		return
	}
	if len(bs) >= directlyReadThreshold {
		// If the data outside the buffer is greater than the threshold,
		// directly call Read to reducing copying overhead.
		n, r.err = r.rd.Read(bs)
	} else {
		r.acquire(1, false) // try read 1 byte
		n = copy(bs, r.buf[r.ri:])
		r.ri += n
	}
	r.rn += n
	err = r.err
	return
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
		if cap(r.buf) > 0 {
			mcache.Free(r.buf)
		}
		r.buf = nil
		r.ri = 0
	}
	r.maxSizeStats.update(r.rn)
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
	_maxSize  int
}

func (s *maxSizeStats) update(size int) {
	s.buckets[s.bucketIdx] = size
	s.bucketIdx = (s.bucketIdx + 1) % statsBucketNum
	var maxSize int
	for _, size := range s.buckets {
		if maxSize < size {
			maxSize = size
		}
	}
	s._maxSize = maxSize
}

func (s *maxSizeStats) maxSize() int {
	return s._maxSize
}
