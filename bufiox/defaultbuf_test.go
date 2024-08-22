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
	"fmt"
	"io"
	"testing"
)

type mockReader struct {
	dataSize int
}

func (r *mockReader) Read(p []byte) (n int, err error) {
	n = r.dataSize
	if n > len(p) {
		n = len(p)
	}
	if n == 0 {
		return 0, io.EOF
	}
	for i := range p[:n] {
		p[i] = byte(0xff)
	}
	r.dataSize -= n
	return
}

func TestDefaultReader(t *testing.T) {
	tcases := []struct {
		dataSize int
		handle   func(reader Reader)
	}{
		{
			dataSize: 1024,
			handle: func(reader Reader) {
				buf, err := reader.Next(1024)
				if err != nil {
					t.Fatal(err)
				}
				for _, b := range buf {
					if b != 0xff {
						t.Fatal("data not equal")
					}
				}
			},
		},
		{
			dataSize: 1024,
			handle: func(reader Reader) {
				buf, err := reader.Next(1025)
				if err != io.EOF {
					t.Fatal("err is not io.EOF", err)
				}
				if buf != nil {
					t.Fatal("buf is not nil")
				}
			},
		},
		{
			dataSize: 1024 * 16,
			handle: func(reader Reader) {
				buf, err := reader.Next(1024)
				if err != nil {
					t.Fatal(err)
				}
				for _, b := range buf {
					if b != 0xff {
						t.Fatal("data not equal")
					}
				}
				if reader.ReadLen() != 1024 {
					t.Fatal("read len is not 1024")
				}
				buf, err = reader.Next(1024 * 14)
				if err != nil {
					t.Fatal(err)
				}
				for _, b := range buf {
					if b != 0xff {
						t.Fatal("data not equal")
					}
				}
				if reader.ReadLen() != 1024*15 {
					t.Fatal("read len is not 1024*15")
				}
				err = reader.Release(nil)
				if err != nil {
					t.Fatal(err)
				}
				if reader.ReadLen() != 0 {
					t.Fatal("read len is not 0")
				}
				buf, err = reader.Peek(1024)
				if err != nil {
					t.Fatal(err)
				}
				for _, b := range buf {
					if b != 0xff {
						t.Fatal("data not equal")
					}
				}
				if reader.ReadLen() != 0 {
					t.Fatal("read len is not 0")
				}
				err = reader.Skip(1024)
				if err != nil {
					t.Fatal(err)
				}
				if reader.ReadLen() != 1024 {
					t.Fatal("read len is not 1024")
				}
				err = reader.Release(nil)
				if err != nil {
					t.Fatal(err)
				}
				switch r := reader.(type) {
				case *DefaultReader:
					if r.buf != nil {
						t.Fatal("buf is not nil")
					}
				case *BytesReader:
					if r.buf != nil {
						t.Fatal("buf is not nil")
					}
				}
				_, err = reader.Next(1)
				if err != io.EOF {
					t.Fatal("err is not io.EOF", err)
				}
				_, err = reader.Peek(1)
				if err != io.EOF {
					t.Fatal("err is not io.EOF", err)
				}
				err = reader.Skip(1)
				if err != io.EOF {
					t.Fatal("err is not io.EOF", err)
				}
			},
		},
	}
	for _, tcase := range tcases {
		r := NewDefaultReader(&mockReader{dataSize: tcase.dataSize})
		tcase.handle(r)

		buf := make([]byte, tcase.dataSize)
		for i := range buf {
			buf[i] = 0xff
		}
		br := NewBytesReader(buf)
		tcase.handle(br)
	}
}

type mockWriter struct {
	dataSize int
}

func (w *mockWriter) Write(p []byte) (n int, err error) {
	if w.dataSize != len(p) {
		return 0, fmt.Errorf("length is not %d", w.dataSize)
	}
	for _, b := range p {
		if b != 0xff {
			return 0, errors.New("data not equal")
		}
	}
	return len(p), nil
}

func setBytes(b []byte, v byte) {
	for i := range b {
		b[i] = v
	}
}

func TestDefaultWriter(t *testing.T) {
	tcases := []struct {
		dataSize int
		handle   func(writer Writer)
	}{
		{
			dataSize: 1024 * 18,
			handle: func(writer Writer) {
				buf, err := writer.Malloc(1024)
				if err != nil {
					t.Fatal(err)
				}
				if writer.WrittenLen() != 1024 {
					t.Fatal("written len is not 1024")
				}
				buf1, err := writer.Malloc(1024)
				if err != nil {
					t.Fatal(err)
				}
				if writer.WrittenLen() != 1024*2 {
					t.Fatal("written len is not 1024*2")
				}
				buf2, err := writer.Malloc(1024 * 4)
				if err != nil {
					t.Fatal(err)
				}
				if writer.WrittenLen() != 1024*6 {
					t.Fatal("written len is not 1024*6")
				}
				buf3, err := writer.Malloc(1024 * 12)
				if err != nil {
					t.Fatal(err)
				}
				if writer.WrittenLen() != 1024*18 {
					t.Fatal("written len is not 1024*18")
				}
				setBytes(buf3, 0xff)
				setBytes(buf2, 0xff)
				setBytes(buf1, 0xff)
				setBytes(buf, 0xff)
				if err = writer.Flush(); err != nil {
					t.Fatal(err)
				}
			},
		},
	}
	for _, tcase := range tcases {
		w := NewDefaultWriter(&mockWriter{dataSize: tcase.dataSize})
		tcase.handle(w)

		buf := make([]byte, 0, defaultBufSize)
		bw := NewBytesWriter(&buf)
		tcase.handle(bw)
		if len(buf) != tcase.dataSize {
			t.Fatal("write data size is not equal!")
		}
	}
}
