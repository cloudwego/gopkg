package xbuf

import "testing"

func TestReadBuf(t *testing.T) {
	x := &XReadBuffer{}

	x.ReadN(10)

	x.CopyBytes(make([]byte, 0))
}
