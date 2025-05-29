package xbuf

import (
	"errors"
	"sync"

	"github.com/bytedance/gopkg/lang/mcache"
)

var (
	ErrXReadBufferNotEnough = errors.New("error xread buffer not enough")
	xreadBufferPool         = sync.Pool{
		New: func() interface{} {
			return &XReadBuffer{
				pool: make([][]byte, 0, 16),
			}
		},
	}
)

type XReadBuffer struct {
	off  int
	buf  []byte
	bufs [][]byte
	pool [][]byte
}

func NewXReadBuffer(bufs [][]byte) *XReadBuffer {
	rb := xreadBufferPool.Get().(*XReadBuffer)
	rb.buf = bufs[0]
	rb.bufs = bufs[1:]
	return rb
}

// ReadN read n bytes from buffer, if buf is not enough, it will read from next buffer.
//
// MAKE SURE IT CAN BE INLINE:
// `can inline (*XReadBuffer).ReadN with cost 80`
func (b *XReadBuffer) ReadN(n int) (buf []byte) {
	buf = b.buf[b.off:]
	if len(buf) < n {
		buf = b.readSlow(n)
	} else {
		b.off += n
	}
	return
}

func (b *XReadBuffer) readSlow(n int) (buf []byte) {
	buf = mcache.Malloc(n)
	b.pool = append(b.pool, buf)
	var l, m int
	if len(b.buf)-b.off > 0 {
		m = copy(buf[l:], b.buf[b.off:])
		l += m
	}
	for l < n {
		if len(b.bufs) == 0 {
			panic(ErrXReadBufferNotEnough.Error())
		}
		b.buf = b.bufs[0]
		b.off = 0
		b.bufs = b.bufs[1:]
		m = copy(buf[l:], b.buf)
		l += m
	}
	b.off += m
	return
}

// CopyBytes copy bytes from buffer, if buf is not enough, it will copy from next buffer.
//
// MAKE SURE IT CAN BE INLINE:
// `can inline (*XReadBuffer).CopyBytes with cost 80`
func (b *XReadBuffer) CopyBytes(buf []byte) {
	n := copy(buf, b.buf[b.off:])
	if len(buf) > n {
		b.copySlow(buf)
	} else {
		b.off += n
	}
}

func (b *XReadBuffer) copySlow(buf []byte) {
	m := len(b.buf) - b.off
	l := m
	for l < len(buf) {
		if len(b.bufs) == 0 {
			panic(ErrXReadBufferNotEnough.Error())
		}
		b.buf = b.bufs[0]
		b.off = 0
		b.bufs = b.bufs[1:]
		m = copy(buf[l:], b.buf)
		l += m
	}
	b.off += m
}

func (b *XReadBuffer) Free() {
	b.off = 0
	b.buf = nil
	b.bufs = nil
	for i := range b.pool {
		mcache.Free(b.pool[i])
		b.pool[i] = nil
	}
	b.pool = b.pool[:0]
	xreadBufferPool.Put(b)
}
