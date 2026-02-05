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

func TestWriteBuffer_Inline(t *testing.T) {
	var b *WriteBuffer

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("should panic")
		}
		stack := string(debug.Stack())
		if !strings.Contains(stack, "MallocN(...)") {
			t.Fatal("should inline MallocN")
		}
	}()

	b.MallocN(10)
}

func TestWriteBuffer_CrossPad(t *testing.T) {
	b := NewWriteBuffer()
	defer b.Free()
	buf := b.MallocN(padLength - 1)
	for i := range buf {
		buf[i] = 'a'
	}
	buf = b.MallocN(2)
	for i := range buf {
		buf[i] = 'b'
	}
	bytes := b.Bytes()
	if len(bytes) != 2 {
		t.Fatal("bytes length should be 2")
	}
	if len(bytes[0]) != padLength-1 {
		t.Fatal("bytes[0] length should be padLength-1")
	}
	for i := range bytes[0] {
		if bytes[0][i] != 'a' {
			t.Fatal("bytes[0][i] should be 'a'")
		}
	}
	if len(bytes[1]) != 2 {
		t.Fatal("bytes[1] length should be 2")
	}
	for i := range bytes[1] {
		if bytes[1][i] != 'b' {
			t.Fatal("bytes[1][i] should be 'b'")
		}
	}
}

func TestWriteBuffer_NoCrossPad(t *testing.T) {
	b := NewWriteBuffer()
	defer b.Free()
	buf := b.MallocN(1024)
	for i := range buf {
		buf[i] = 'a'
	}
	buf = b.MallocN(1024)
	for i := range buf {
		buf[i] = 'b'
	}
	bytes := b.Bytes()
	if len(bytes) != 1 {
		t.Fatal("bytes length should be 1")
	}
	if len(bytes[0]) != 2048 {
		t.Fatal("bytes[0] length should be 2048")
	}
	for i := range bytes[0] {
		if i < 1024 && bytes[0][i] != 'a' {
			t.Fatal("bytes[0][i] should be 'a'")
		}
		if i >= 1024 && bytes[0][i] != 'b' {
			t.Fatal("bytes[0][i] should be 'b'")
		}
	}
}

func TestWriteBuffer_WriteDirect(t *testing.T) {
	b := NewWriteBuffer()
	defer b.Free()
	buf := b.MallocN(1024)
	for i := range buf {
		buf[i] = 'a'
	}
	b.WriteDirect([]byte{'b', 'c'})
	bytes := b.Bytes()
	if len(bytes) != 2 {
		t.Fatal("bytes length should be 2")
	}
	if len(bytes[0]) != 1024 {
		t.Fatal("bytes[0] length should be 1024")
	}
	if len(bytes[1]) != 2 {
		t.Fatal("bytes[1] length should be 2")
	}
	for i := range bytes[0] {
		if bytes[0][i] != 'a' {
			t.Fatal("bytes[0][i] should be 'a'")
		}
	}
	for i := range bytes[1] {
		if bytes[1][i] != byte('b'+i) {
			t.Fatal("bytes[1][i] should be 'b'+i")
		}
	}
}

func BenchmarkWriteBuf_MallocN(b *testing.B) {
	x := NewWriteBuffer()
	defer x.Free()

	var tmp []byte
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tmp = x.MallocN(1)
	}
	_ = tmp
}

func BenchmarkBytes_Write(b *testing.B) {
	bytes := make([]byte, b.N)
	var off int
	var tmp []byte
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tmp = bytes[off : off+1]
		off++
	}
	_ = tmp
}
