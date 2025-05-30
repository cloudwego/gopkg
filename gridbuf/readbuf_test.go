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
	"runtime/debug"
	"strings"
	"testing"
)

func TestReadBuf_Inline(t *testing.T) {
	var x *ReadBuffer

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("should panic")
		}
		stack := string(debug.Stack())
		if !strings.Contains(stack, "ReadN(...)") {
			t.Fatal("should inline ReadN")
		}
	}()
	x.ReadN(10)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("should panic")
		}
		stack := string(debug.Stack())
		if !strings.Contains(stack, "CopyBytes(...)") {
			t.Fatal("should inline CopyBytes")
		}
	}()

	x.CopyBytes(make([]byte, 1))
}

func TestReadBuf_CrossPad(t *testing.T) {
	tf := func(getBuf func(x *ReadBuffer, n int) []byte) {
		ori1 := make([]byte, padLength)
		for i := range ori1 {
			ori1[i] = 'a'
		}
		ori2 := make([]byte, padLength)
		for i := range ori2 {
			ori2[i] = 'b'
		}
		ori := [][]byte{ori1, ori2}
		x := NewReadBuffer(ori)
		defer x.Free()
		buf := getBuf(x, padLength-1)
		if len(buf) < padLength-1 {
			t.Fatal("buf length should be great or equal to padLength-1")
		}
		for _, byt := range buf[:padLength-1] {
			if byt != 'a' {
				t.Fatal("byt should be 'a'")
			}
		}
		buf = getBuf(x, 2)
		if len(buf) < 2 {
			t.Fatal("buf length should be great or equal to 2")
		}
		if buf[0] != 'a' {
			t.Fatal("buf[0] should be 'a'")
		}
		if buf[1] != 'b' {
			t.Fatal("buf[1] should be 'b'")
		}
		buf = getBuf(x, padLength-1)
		if len(buf) < padLength-1 {
			t.Fatal("buf length should be great or equal to padLength-1")
		}
		for _, byt := range buf[:padLength-1] {
			if byt != 'b' {
				t.Fatal("byt should be 'b'")
			}
		}
	}
	tf(func(x *ReadBuffer, n int) []byte {
		return x.ReadN(n)
	})
	tf(func(x *ReadBuffer, n int) []byte {
		buf := make([]byte, n)
		x.CopyBytes(buf)
		return buf
	})
}

func TestReadBuf_NoCrossPad(t *testing.T) {
	tf := func(getBuf func(x *ReadBuffer, n int) []byte) {
		ori1 := make([]byte, padLength/2)
		for i := range ori1 {
			ori1[i] = 'a'
		}
		ori2 := make([]byte, padLength/2)
		for i := range ori2 {
			ori2[i] = 'b'
		}
		ori := append(ori1, ori2...)
		x := NewReadBuffer([][]byte{ori})
		defer x.Free()

		buf := getBuf(x, padLength/2)
		if len(buf) < padLength/2 {
			t.Fatal("buf length should be great or equal to padLength/2")
		}
		for _, byt := range buf[:padLength/2] {
			if byt != 'a' {
				t.Fatal("byt should be 'a'")
			}
		}
		buf = getBuf(x, padLength/2)
		if len(buf) < padLength/2 {
			t.Fatal("buf length should be great or equal to padLength/2")
		}
		for _, byt := range buf[:padLength/2] {
			if byt != 'b' {
				t.Fatal("byt should be 'b'")
			}
		}
	}
	tf(func(x *ReadBuffer, n int) []byte {
		return x.ReadN(n)
	})
	tf(func(x *ReadBuffer, n int) []byte {
		buf := make([]byte, n)
		x.CopyBytes(buf)
		return buf
	})
}
