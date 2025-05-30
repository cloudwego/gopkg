package xbuf

import (
	"runtime/debug"
	"strings"
	"testing"
)

func TestXWriteBuffer_Inline(t *testing.T) {
	var b *XWriteBuffer

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

func TestXWriteBuffer_CrossPad(t *testing.T) {
	b := NewXWriteBuffer()
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

func TestXWriteBuffer_NoCrossPad(t *testing.T) {
	b := NewXWriteBuffer()
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

func TestXWriteBuffer_WriteDirect(t *testing.T) {
	b := NewXWriteBuffer()
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
