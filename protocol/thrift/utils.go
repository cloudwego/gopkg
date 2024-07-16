package thrift

import (
	"io"
	"sync"
	"unsafe"
)

// p2i32, used by skipType which implements a fast skip with unsafe.Pointer without bounds check
func p2i32(p unsafe.Pointer) int32 {
	return int32(uint32(*(*byte)(unsafe.Add(p, 3))) |
		uint32(*(*byte)(unsafe.Add(p, 2)))<<8 |
		uint32(*(*byte)(unsafe.Add(p, 1)))<<16 |
		uint32(*(*byte)(p))<<24)
}

// nextReader provides a wrapper for io.Reader to use BinaryReader
type nextReader struct {
	r io.Reader
	b [4096]byte
}

var poolNextReader = sync.Pool{
	New: func() interface{} {
		return &nextReader{}
	},
}

// Next implements nextIface of BinaryReader
func (r *nextReader) Next(n int) ([]byte, error) {
	b := r.b[:]
	if n <= len(b) {
		b = b[:n]
	} else {
		b = make([]byte, n)
	}
	_, err := io.ReadFull(r.r, b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// Discard implements discardIface of BinaryReader
func (r *nextReader) Discard(n int) (int, error) {
	ret := 0
	b := r.b[:]
	for n > 0 {
		if len(b) > n {
			b = b[:n]
		}
		readn, err := r.r.Read(b)
		ret += readn
		if err != nil {
			return ret, err
		}
		n -= readn
	}
	return ret, nil
}

// Reset ... for reusing nextReader
func (r *nextReader) Reset(rd io.Reader) {
	r.r = rd
}
