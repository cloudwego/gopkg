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

package bufiox

import (
	"errors"

	"github.com/bytedance/gopkg/lang/dirtmake"
)

var errNoRemainingData = errors.New("bufiox: no remaining data left")

var _ Reader = &BytesReader{}

type BytesReader struct {
	buf []byte // buf[ri:] is the buffer for reading.
	ri  int    // buf read positions
}

// NewBytesReader returns a new DefaultReader that reads from buf[:len(buf)].
// Its operation on buf is read-only.
func NewBytesReader(buf []byte) *BytesReader {
	r := &BytesReader{buf: buf}
	return r
}

func (r *BytesReader) Next(n int) (buf []byte, err error) {
	if n < 0 {
		err = errNegativeCount
		return
	}
	if n > len(r.buf)-r.ri {
		err = errNoRemainingData
		return
	}
	// nocopy read
	buf = r.buf[r.ri : r.ri+n]
	r.ri += n
	return
}

func (r *BytesReader) Peek(n int) (buf []byte, err error) {
	if n < 0 {
		err = errNegativeCount
		return
	}
	if n > len(r.buf)-r.ri {
		err = errNoRemainingData
		return
	}
	// nocopy read
	buf = r.buf[r.ri : r.ri+n]
	return
}

func (r *BytesReader) Skip(n int) (err error) {
	if n < 0 {
		err = errNegativeCount
		return
	}
	if n > len(r.buf)-r.ri {
		err = errNoRemainingData
		return
	}
	r.ri += n
	return
}

func (r *BytesReader) ReadLen() (n int) {
	return r.ri
}

func (r *BytesReader) ReadBinary(bs []byte) (n int, err error) {
	if len(bs) > len(r.buf)-r.ri {
		err = errNoRemainingData
		return
	}
	n = copy(bs, r.buf[r.ri:])
	r.ri += n
	return
}

func (r *BytesReader) Release(e error) error {
	r.buf = r.buf[r.ri:]
	r.ri = 0
	return nil
}

type BytesWriter struct {
	buf   []byte
	toBuf *[]byte
}

// NewBytesWriter returns a new DefaultWriter that writes to buf[len(buf):cap(buf)].
// The WrittenLen is set to len(buf) before the first write.
func NewBytesWriter(buf *[]byte) *BytesWriter {
	w := &BytesWriter{toBuf: buf}
	return w
}

func (w *BytesWriter) acquire(n int) {
	// fast path, for inline
	if len(w.buf)+n <= cap(w.buf) {
		return
	}
	w.acquireSlow(n)
}

func (w *BytesWriter) acquireSlow(n int) {
	if cap(w.buf) == 0 {
		maxSize := defaultBufSize
		for ; maxSize < n; maxSize *= 2 {
		}
		w.buf = dirtmake.Bytes(0, maxSize)
	}

	if n > cap(w.buf)-len(w.buf) {
		// grow buffer
		var ncap int
		// reserve the length of len(w.buf) for copying data from the old buffer during flush
		for ncap = cap(w.buf) * 2; ncap-len(w.buf) < n; ncap *= 2 {
		}
		nbuf := dirtmake.Bytes(ncap, ncap)
		w.buf = nbuf[:copy(nbuf, w.buf)]
	}
}

func (w *BytesWriter) Malloc(n int) (buf []byte, err error) {
	if n < 0 {
		err = errNegativeCount
		return
	}
	w.acquire(n)
	buf = w.buf[len(w.buf) : len(w.buf)+n]
	w.buf = w.buf[:len(w.buf)+n]
	return
}

func (w *BytesWriter) WriteBinary(bs []byte) (n int, err error) {
	w.acquire(len(bs))
	n = copy(w.buf[len(w.buf):cap(w.buf)], bs)
	w.buf = w.buf[:len(w.buf)+n]
	return
}

func (w *BytesWriter) WrittenLen() int {
	return len(w.buf)
}

func (w *BytesWriter) Flush() (err error) {
	if len(w.buf) == 0 {
		*w.toBuf = []byte{}
		return nil
	}
	*w.toBuf = w.buf[:len(w.buf):len(w.buf)]
	w.buf = nil
	return nil
}
