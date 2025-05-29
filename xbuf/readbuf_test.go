package xbuf

import (
	"runtime/debug"
	"strings"
	"testing"
)

func TestReadBuf(t *testing.T) {
	var x *XReadBuffer

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
