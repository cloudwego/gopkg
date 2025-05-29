package thrift

import (
	"encoding/binary"
	"math"
	"sync"

	"github.com/bytedance/gopkg/lang/mcache"

	"github.com/cloudwego/gopkg/unsafex"
)

const padLength = 1 << 13

var xwriteBufferPool = sync.Pool{
	New: func() interface{} {
		return &XWriteBuffer{
			bufs: make([][]byte, 0, 16),
			pool: make([][]byte, 0, 16),
		}
	},
}

type XWriteBuffer struct {
	off  int // write offset of buf
	buf  []byte
	bufs [][]byte
	pool [][]byte
}

func NewXWriteBuffer() *XWriteBuffer {
	return xwriteBufferPool.Get().(*XWriteBuffer)
}

func (b *XWriteBuffer) Bytes() [][]byte {
	if b.off > 0 {
		b.bufs = append(b.bufs, b.buf[:b.off])
		b.buf = b.buf[b.off:]
		b.off = 0
	}
	return b.bufs
}

func (b *XWriteBuffer) Free() {
	b.off = 0
	b.buf = nil
	for i := range b.bufs {
		b.bufs[i] = nil
	}
	b.bufs = b.bufs[:0]
	for i := range b.pool {
		mcache.Free(b.pool[i])
		b.pool[i] = nil
	}
	b.pool = b.pool[:0]
	xwriteBufferPool.Put(b)
}

func (b *XWriteBuffer) grow(n int) {
	if b.off > 0 {
		b.buf = b.buf[:b.off]
		b.bufs = append(b.bufs, b.buf)
		b.off = 0
	}
	// refresh buf
	buf := mcache.Malloc(n, n)
	buf = buf[:cap(buf)]
	b.pool = append(b.pool, buf)
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

func (XBufferProtocol) WriteMessageBegin(b *XWriteBuffer, name string, typeID TMessageType, seq int32) {
	if len(b.buf)-b.off < 4+(4+len(name))+4 {
		b.grow(maxInt(padLength, 4+(4+len(name))+4))
	}
	binary.BigEndian.PutUint32(b.buf[b.off:], uint32(msgVersion1)|uint32(typeID&msgTypeMask))
	binary.BigEndian.PutUint32(b.buf[b.off+4:], uint32(len(name)))
	off := 8 + copy(b.buf[b.off+8:], name)
	binary.BigEndian.PutUint32(b.buf[off:], uint32(seq))
	b.off = off + 4
}

func (XBufferProtocol) WriteFieldBegin(b *XWriteBuffer, typeID TType, id int16) {
	if len(b.buf)-b.off < 3 {
		b.grow(padLength)
	}
	b.buf[b.off] = byte(typeID)
	binary.BigEndian.PutUint16(b.buf[b.off+1:], uint16(id))
	b.off += 3
}

func (XBufferProtocol) WriteFieldStop(b *XWriteBuffer) {
	if len(b.buf)-b.off < 1 {
		b.grow(padLength)
	}
	b.buf[b.off] = byte(STOP)
	b.off += 1
}

func (XBufferProtocol) WriteMapBegin(b *XWriteBuffer, kt, vt TType, size int) {
	if len(b.buf)-b.off < 6 {
		b.grow(padLength)
	}
	b.buf[b.off] = byte(kt)
	b.buf[b.off+1] = byte(vt)
	binary.BigEndian.PutUint32(b.buf[b.off+2:], uint32(size))
	b.off += 6
}

func (XBufferProtocol) WriteListBegin(b *XWriteBuffer, et TType, size int) {
	if len(b.buf)-b.off < 5 {
		b.grow(padLength)
	}
	b.buf[b.off] = byte(et)
	binary.BigEndian.PutUint32(b.buf[b.off+1:], uint32(size))
	b.off += 5
}

func (XBufferProtocol) WriteSetBegin(b *XWriteBuffer, et TType, size int) {
	if len(b.buf)-b.off < 5 {
		b.grow(padLength)
	}
	b.buf[b.off] = byte(et)
	binary.BigEndian.PutUint32(b.buf[b.off+1:], uint32(size))
	b.off += 5
}

func (XBufferProtocol) WriteBool(b *XWriteBuffer, v bool) {
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

func (XBufferProtocol) WriteByte(b *XWriteBuffer, v int8) {
	if len(b.buf)-b.off < 1 {
		b.grow(padLength)
	}
	b.buf[b.off] = byte(v)
	b.off += 1
}

func (XBufferProtocol) WriteI16(b *XWriteBuffer, v int16) {
	if len(b.buf)-b.off < 2 {
		b.grow(padLength)
	}
	binary.BigEndian.PutUint16(b.buf[b.off:], uint16(v))
	b.off += 2
}

func (XBufferProtocol) WriteI32(b *XWriteBuffer, v int32) {
	if len(b.buf)-b.off < 4 {
		b.grow(padLength)
	}
	binary.BigEndian.PutUint32(b.buf[b.off:], uint32(v))
	b.off += 4
}

func (XBufferProtocol) WriteI64(b *XWriteBuffer, v int64) {
	if len(b.buf)-b.off < 8 {
		b.grow(padLength)
	}
	binary.BigEndian.PutUint64(b.buf[b.off:], uint64(v))
	b.off += 8
}

func (XBufferProtocol) WriteDouble(b *XWriteBuffer, v float64) {
	if len(b.buf)-b.off < 8 {
		b.grow(padLength)
	}
	binary.BigEndian.PutUint64(b.buf[b.off:], math.Float64bits(v))
	b.off += 8
}

func (p XBufferProtocol) writeDirect(b *XWriteBuffer, v []byte) {
	// write header
	if len(b.buf)-b.off < 4 {
		b.grow(padLength)
	}
	binary.BigEndian.PutUint32(b.buf[b.off:], uint32(len(v)))
	b.off += 4

	// relink buffers
	b.bufs = append(b.bufs, b.buf[:b.off])
	b.buf = b.buf[b.off:]
	b.off = 0

	// write directly
	b.bufs = append(b.bufs, v)
}

func (p XBufferProtocol) WriteBinary(b *XWriteBuffer, v []byte) {
	if len(v) >= nocopyWriteThreshold {
		p.writeDirect(b, v)
		return
	}
	if len(b.buf)-b.off < 4+len(v) {
		b.grow(maxInt(padLength, 4+len(v)))
	}
	binary.BigEndian.PutUint32(b.buf[b.off:], uint32(len(v)))
	b.off += 4 + copy(b.buf[b.off+4:], v)
}

func (p XBufferProtocol) WriteString(b *XWriteBuffer, v string) {
	if len(v) >= nocopyWriteThreshold {
		p.writeDirect(b, unsafex.StringToBinary(v))
		return
	}
	if len(b.buf)-b.off < 4+len(v) {
		b.grow(maxInt(padLength, 4+len(v)))
	}
	binary.BigEndian.PutUint32(b.buf[b.off:], uint32(len(v)))
	b.off += 4 + copy(b.buf[b.off+4:], v)
}

func (p XBufferProtocol) RawWrite(b *XWriteBuffer, v []byte) {
	if len(v) >= nocopyWriteThreshold {
		// relink buffers
		if b.off > 0 {
			b.bufs = append(b.bufs, b.buf[:b.off])
			b.buf = b.buf[b.off:]
			b.off = 0
		}

		// write directly
		b.bufs = append(b.bufs, v)
		return
	}
	if len(b.buf)-b.off < len(v) {
		b.grow(maxInt(padLength, len(v)))
	}
	b.off += copy(b.buf[b.off:], v)
}
