package thrift

import (
	"encoding/binary"
	"math"

	"github.com/cloudwego/gopkg/unsafex"
)

const padLength = 4096

type BufferPool interface {
	Get(n int) []byte
	Free()
}

type WriteBuffer struct {
	off  int // write offset of buf
	buf  []byte
	bufs [][]byte
	pool BufferPool
}

func (b *WriteBuffer) Freeze() ([][]byte, BufferPool) {
	if b.off > 0 {
		b.bufs = append(b.bufs, b.buf[:b.off])
		b.buf = b.buf[b.off:]
		b.off = 0
	}
	return b.bufs, b.pool
}

func (b *WriteBuffer) grow(n int) {
	b.buf = b.buf[:b.off]
	b.bufs = append(b.bufs, b.buf)
	b.off = 0
	// refresh buf
	buf := b.pool.Get(n)
	b.buf = buf
}

func maxInt(x, y int) int {
	if x > y {
		return x
	}
	return y
}

var XBuffer XBufferProtocol

type XBufferProtocol struct{}

func (XBufferProtocol) WriteMessageBegin(b *WriteBuffer, name string, typeID TMessageType, seq int32) {
	if len(b.buf)-b.off < 4+(4+len(name))+4 {
		b.grow(maxInt(padLength, 4+(4+len(name))+4))
	}
	binary.BigEndian.PutUint32(b.buf[b.off:], uint32(msgVersion1)|uint32(typeID&msgTypeMask))
	binary.BigEndian.PutUint32(b.buf[b.off+4:], uint32(len(name)))
	off := 8 + copy(b.buf[b.off+8:], name)
	binary.BigEndian.PutUint32(b.buf[off:], uint32(seq))
	b.off = off + 4
}

func (XBufferProtocol) WriteFieldBegin(b *WriteBuffer, typeID TType, id int16) {
	if len(b.buf)-b.off < 3 {
		b.grow(padLength)
	}
	b.buf[b.off] = byte(typeID)
	binary.BigEndian.PutUint16(b.buf[b.off+1:], uint16(id))
	b.off += 3
}

func (XBufferProtocol) WriteFieldStop(b *WriteBuffer) {
	if len(b.buf)-b.off < 1 {
		b.grow(padLength)
	}
	b.buf[b.off] = byte(STOP)
	b.off += 1
}

func (XBufferProtocol) WriteMapBegin(b *WriteBuffer, kt, vt TType, size int) {
	if len(b.buf)-b.off < 6 {
		b.grow(padLength)
	}
	b.buf[b.off] = byte(kt)
	b.buf[b.off+1] = byte(vt)
	binary.BigEndian.PutUint32(b.buf[b.off+2:], uint32(size))
	b.off += 6
}

func (XBufferProtocol) WriteListBegin(b *WriteBuffer, et TType, size int) {
	if len(b.buf)-b.off < 5 {
		b.grow(padLength)
	}
	b.buf[b.off] = byte(et)
	binary.BigEndian.PutUint32(b.buf[b.off+1:], uint32(size))
	b.off += 5
}

func (XBufferProtocol) WriteSetBegin(b *WriteBuffer, et TType, size int) {
	if len(b.buf)-b.off < 5 {
		b.grow(padLength)
	}
	b.buf[b.off] = byte(et)
	binary.BigEndian.PutUint32(b.buf[b.off+1:], uint32(size))
	b.off += 5
}

func (XBufferProtocol) WriteBool(b *WriteBuffer, v bool) {
	if len(b.buf)-b.off < 1 {
		b.grow(padLength)
	}
	if v {
		b.buf[b.off] = 1
	} else {
		b.buf[b.off] = 0
	}
	b.off += 1
}

func (XBufferProtocol) WriteByte(b *WriteBuffer, v int8) {
	if len(b.buf)-b.off < 1 {
		b.grow(padLength)
	}
	b.buf[b.off] = byte(v)
	b.off += 1
}

func (XBufferProtocol) WriteI16(b *WriteBuffer, v int16) {
	if len(b.buf)-b.off < 2 {
		b.grow(padLength)
	}
	binary.BigEndian.PutUint16(b.buf[b.off:], uint16(v))
	b.off += 2
}

func (XBufferProtocol) WriteI32(b *WriteBuffer, v int32) {
	if len(b.buf)-b.off < 4 {
		b.grow(padLength)
	}
	binary.BigEndian.PutUint32(b.buf[b.off:], uint32(v))
	b.off += 4
}

func (XBufferProtocol) WriteI64(b *WriteBuffer, v int64) {
	if len(b.buf)-b.off < 8 {
		b.grow(padLength)
	}
	binary.BigEndian.PutUint64(b.buf[b.off:], uint64(v))
	b.off += 8
}

func (XBufferProtocol) WriteDouble(b *WriteBuffer, v float64) {
	if len(b.buf)-b.off < 8 {
		b.grow(padLength)
	}
	binary.BigEndian.PutUint64(b.buf[b.off:], math.Float64bits(v))
	b.off += 8
}

func (XBufferProtocol) writeDirect(b *WriteBuffer, v []byte) {
	if b.off > 0 {
		b.bufs = append(b.bufs, b.buf[:b.off])
		b.buf = b.buf[b.off:]
		b.off = 0
	}
	b.bufs = append(b.bufs, v)
}

func (p XBufferProtocol) WriteBinary(b *WriteBuffer, v []byte) {
	if len(v) >= nocopyWriteThreshold {
		p.writeDirect(b, v)
		return
	}
	if len(b.buf)-b.off < 4+len(v) {
		b.grow(maxInt(padLength, 4+len(v)))
	}
	binary.BigEndian.PutUint32(b.buf[b.off:], uint32(len(v)))
	b.off = 4 + copy(b.buf[b.off+4:], v)
}

func (p XBufferProtocol) WriteString(b *WriteBuffer, v string) {
	if len(v) >= nocopyWriteThreshold {
		p.writeDirect(b, unsafex.StringToBinary(v))
		return
	}
	if len(b.buf)-b.off < 4+len(v) {
		b.grow(maxInt(padLength, 4+len(v)))
	}
	binary.BigEndian.PutUint32(b.buf[b.off:], uint32(len(v)))
	b.off = 4 + copy(b.buf[b.off+4:], v)
}
