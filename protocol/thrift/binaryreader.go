package thrift

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"sync"
)

type nextIface interface {
	Next(n int) ([]byte, error)
}

type discardIface interface {
	Discard(n int) (int, error)
}

// BinaryReader represents a reader for binary protocol
type BinaryReader struct {
	r nextIface
	d discardIface

	rn int64
}

var poolBinaryReader = sync.Pool{
	New: func() interface{} {
		return &BinaryReader{}
	},
}

// NewBinaryReader ... call Release if no longer use for reusing
func NewBinaryReader(r io.Reader) *BinaryReader {
	ret := poolBinaryReader.Get().(*BinaryReader)
	ret.reset()
	if nextr, ok := r.(nextIface); ok {
		ret.r = nextr
	} else {
		nextr := poolNextReader.Get().(*nextReader)
		nextr.Reset(r)
		ret.r = nextr
		ret.d = nextr
	}
	return ret
}

// Release ...
func (r *BinaryReader) Release() {
	nextr, ok := r.r.(*nextReader)
	if ok {
		poolNextReader.Put(nextr)
	}
	r.reset()
	poolBinaryReader.Put(r)
}

func (r *BinaryReader) reset() {
	r.r = nil
	r.d = nil
	r.rn = 0
}

func (r *BinaryReader) next(n int) (b []byte, err error) {
	b, err = r.r.Next(n)
	if err != nil {
		err = NewProtocolExceptionWithErr(err)
	}
	r.rn += int64(len(b))
	return
}

func (r *BinaryReader) skipn(n int) (err error) {
	if n < 0 {
		return errNegativeSize
	}
	if r.d != nil {
		var sz int
		sz, err = r.d.Discard(n)
		r.rn += int64(sz)
	} else {
		var b []byte
		b, err = r.r.Next(n)
		r.rn += int64(len(b))
	}
	if err != nil {
		return NewProtocolExceptionWithErr(err)
	}
	return nil
}

// Readn returns total bytes read from underlying reader
func (r *BinaryReader) Readn() int64 {
	return r.rn
}

// ReadBool ...
func (r *BinaryReader) ReadBool() (v bool, err error) {
	b, err := r.next(1)
	if err != nil {
		return false, err
	}
	v = b[0] == 1
	return
}

// ReadByte ...
func (r *BinaryReader) ReadByte() (v int8, err error) {
	b, err := r.next(1)
	if err != nil {
		return 0, err
	}
	v = int8(b[0])
	return
}

// ReadI16 ...
func (r *BinaryReader) ReadI16() (v int16, err error) {
	b, err := r.next(2)
	if err != nil {
		return 0, err
	}
	v = int16(binary.BigEndian.Uint16(b))
	return
}

// ReadI32 ...
func (r *BinaryReader) ReadI32() (v int32, err error) {
	b, err := r.next(4)
	if err != nil {
		return 0, err
	}
	v = int32(binary.BigEndian.Uint32(b))
	return
}

// ReadI64 ...
func (r *BinaryReader) ReadI64() (v int64, err error) {
	b, err := r.next(8)
	if err != nil {
		return 0, err
	}
	v = int64(binary.BigEndian.Uint64(b))
	return
}

// ReadDouble ...
func (r *BinaryReader) ReadDouble() (v float64, err error) {
	b, err := r.next(8)
	if err != nil {
		return 0, err
	}
	v = math.Float64frombits(binary.BigEndian.Uint64(b))
	return
}

// ReadBinary ...
func (r *BinaryReader) ReadBinary() (b []byte, err error) {
	sz, err := r.ReadI32()
	if err != nil {
		return nil, err
	}
	b, err = r.next(int(sz))
	b = []byte(string(b)) // copy. use span cache?
	return
}

// ReadString ...
func (r *BinaryReader) ReadString() (s string, err error) {
	sz, err := r.ReadI32()
	if err != nil {
		return "", err
	}
	b, err := r.next(int(sz))
	if err != nil {
		return "", err
	}
	s = string(b) // copy. use span cache?
	return
}

// ReadMessageBegin ...
func (r *BinaryReader) ReadMessageBegin() (name string, typeID TMessageType, seq int32, err error) {
	var header int32
	header, err = r.ReadI32()
	if err != nil {
		return
	}
	// read header for version and type
	if uint32(header)&msgVersionMask != msgVersion1 {
		err = errBadVersion
		return
	}
	typeID = TMessageType(uint32(header) & msgTypeMask)

	// read method name
	name, err = r.ReadString()
	if err != nil {
		return
	}

	// read seq
	seq, err = r.ReadI32()
	if err != nil {
		return
	}
	return
}

// ReadFieldBegin ...
func (r *BinaryReader) ReadFieldBegin() (typeID TType, id int16, err error) {
	b, err := r.next(1)
	if err != nil {
		return 0, 0, err
	}
	typeID = TType(b[0])
	if typeID == STOP {
		return STOP, 0, nil
	}
	b, err = r.next(2)
	if err != nil {
		return 0, 0, err
	}
	id = int16(binary.BigEndian.Uint16(b))
	return
}

// ReadMapBegin ...
func (r *BinaryReader) ReadMapBegin() (kt, vt TType, size int, err error) {
	b, err := r.next(6)
	if err != nil {
		return 0, 0, 0, err
	}
	kt, vt, size = TType(b[0]), TType(b[1]), int(binary.BigEndian.Uint32(b[2:]))
	return
}

// ReadListBegin ...
func (r *BinaryReader) ReadListBegin() (et TType, size int, err error) {
	b, err := r.next(5)
	if err != nil {
		return 0, 0, err
	}
	et, size = TType(b[0]), int(binary.BigEndian.Uint32(b[1:]))
	return
}

// ReadSetBegin ...
func (r *BinaryReader) ReadSetBegin() (et TType, size int, err error) {
	b, err := r.next(5)
	if err != nil {
		return 0, 0, err
	}
	et, size = TType(b[0]), int(binary.BigEndian.Uint32(b[1:]))
	return
}

// Skip ...
func (r *BinaryReader) Skip(t TType) error {
	return r.skipType(t, defaultRecursionDepth)
}

func (r *BinaryReader) skipstr() error {
	n, err := r.ReadI32()
	if err != nil {
		return err
	}
	return r.skipn(int(n))
}

func (r *BinaryReader) skipType(t TType, maxdepth int) error {
	if maxdepth == 0 {
		return errDepthLimitExceeded
	}
	if n := typeToSize[t]; n > 0 {
		return r.skipn(int(n))
	}
	switch t {
	case STRING:
		return r.skipstr()
	case MAP:
		kt, vt, sz, err := r.ReadMapBegin()
		if err != nil {
			return err
		}
		if sz < 0 {
			return errNegativeSize
		}
		ksz, vsz := int(typeToSize[kt]), int(typeToSize[vt])
		if ksz > 0 && vsz > 0 {
			return r.skipn(int(sz) * (ksz + vsz))
		}
		for j := 0; j < sz; j++ {
			if ksz > 0 {
				err = r.skipn(ksz)
			} else if kt == STRING {
				err = r.skipstr()
			} else {
				err = r.skipType(kt, maxdepth-1)
			}
			if err != nil {
				return err
			}
			if vsz > 0 {
				err = r.skipn(vsz)
			} else if vt == STRING {
				err = r.skipstr()
			} else {
				err = r.skipType(vt, maxdepth-1)
			}
			if err != nil {
				return err
			}
		}
		return nil
	case LIST, SET:
		vt, sz, err := r.ReadListBegin()
		if err != nil {
			return err
		}
		if sz < 0 {
			return errNegativeSize
		}
		if vsz := typeToSize[vt]; vsz > 0 {
			return r.skipn(sz * int(vsz))
		}
		for j := 0; j < sz; j++ {
			if vt == STRING {
				err = r.skipstr()
			} else {
				err = r.skipType(vt, maxdepth-1)
			}
			if err != nil {
				return err
			}
		}
		return nil
	case STRUCT:
		for {
			ft, _, err := r.ReadFieldBegin()
			if ft == STOP {
				return nil
			}
			if fsz := typeToSize[ft]; fsz > 0 {
				err = r.skipn(int(fsz))
			} else {
				err = r.skipType(ft, maxdepth-1)
			}
			if err != nil {
				return err
			}
		}
	default:
		return NewProtocolException(INVALID_DATA, fmt.Sprintf("unknown data type %d", t))
	}
}
