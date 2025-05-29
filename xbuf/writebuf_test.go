package xbuf

import (
	"runtime/debug"
	"strings"
	"testing"
)

func TestXWriteBuffer(t *testing.T) {
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
