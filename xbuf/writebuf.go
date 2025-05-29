package xbuf

import (
	"sync"

	"github.com/bytedance/gopkg/lang/mcache"
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

// MallocN malloc n bytes from buffer, if buf is not enough, it will grow.
//
// MAKE SURE IT CAN BE INLINE:
// `can inline (*XWriteBuffer).MallocN with cost 79`
func (b *XWriteBuffer) MallocN(n int) (buf []byte) {
	buf = b.buf[b.off:]
	if len(buf) < n {
		buf = b.growSlow(n)
	}
	b.off += n
	return
}

func (b *XWriteBuffer) growSlow(n int) []byte {
	if b.off > 0 {
		b.buf = b.buf[:b.off]
		b.bufs = append(b.bufs, b.buf)
		b.off = 0
	}
	// refresh buf
	if n < padLength {
		n = padLength
	}
	buf := mcache.Malloc(n)
	buf = buf[:cap(buf)]
	b.pool = append(b.pool, buf)
	b.buf = buf
	return buf
}

func (b *XWriteBuffer) WriteDirect(buf []byte) {
	// relink buffers
	if b.off > 0 {
		b.bufs = append(b.bufs, b.buf[:b.off])
		b.buf = b.buf[b.off:]
		b.off = 0
	}

	// write directly
	b.bufs = append(b.bufs, buf)
	return
}
