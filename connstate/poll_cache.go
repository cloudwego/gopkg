package connstate

import (
	"sync"
	"sync/atomic"
	"unsafe"
)

const pollBlockSize = 4 * 1024

type fdOperator struct {
	link  *fdOperator // in pollcache, protected by pollcache.lock
	index int32

	fd   int
	conn unsafe.Pointer // *connWithState
}

var pollcache pollCache

type pollCache struct {
	lock  sync.Mutex
	first *fdOperator
	cache []*fdOperator
	// freelist store the freeable operator
	// to reduce GC pressure, we only store op index here
	freelist []int32
	freeack  int32
}

func (c *pollCache) alloc() *fdOperator {
	c.lock.Lock()
	if c.first == nil {
		const pdSize = unsafe.Sizeof(fdOperator{})
		n := pollBlockSize / pdSize
		if n == 0 {
			n = 1
		}
		index := int32(len(c.cache))
		for i := uintptr(0); i < n; i++ {
			pd := &fdOperator{index: index}
			c.cache = append(c.cache, pd)
			pd.link = c.first
			c.first = pd
			index++
		}
	}
	op := c.first
	c.first = op.link
	c.lock.Unlock()
	return op
}

// freeable mark the operator that could be freed
// only poller could do the real free action
func (c *pollCache) freeable(op *fdOperator) {
	atomic.StorePointer(&op.conn, nil)
	c.lock.Lock()
	// reset all state
	if atomic.CompareAndSwapInt32(&c.freeack, 1, 0) {
		for _, idx := range c.freelist {
			op := c.cache[idx]
			op.link = c.first
			c.first = op
		}
		c.freelist = c.freelist[:0]
	}
	c.freelist = append(c.freelist, op.index)
	c.lock.Unlock()
}

func (c *pollCache) free() {
	atomic.StoreInt32(&c.freeack, 1)
}
