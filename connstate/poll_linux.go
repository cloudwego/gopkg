package connstate

import (
	"sync/atomic"
	"syscall"
	"unsafe"
)

type epoller struct {
	epfd int
}

func (p *epoller) wait() error {
	for {
		events := make([]epollevent, 32)
		n, err := EpollWait(p.epfd, events, -1)
		if err != nil && err != syscall.EINTR {
			return err
		}
		for i := 0; i < n; i++ {
			ev := &events[i]
			op := *(**fdOperator)(unsafe.Pointer(&ev.data))
			if conn := (*connWithState)(atomic.LoadPointer(&op.conn)); conn != nil {
				if ev.events&(syscall.EPOLLHUP|syscall.EPOLLRDHUP|syscall.EPOLLERR) != 0 {
					atomic.CompareAndSwapUint32(&conn.state, uint32(StateOK), uint32(StateRemoteClosed))
				}
			}
		}
		// we can make sure that there is no op remaining if finished handling all events
		pollcache.free()
	}
}

func (p *epoller) control(fd *fdOperator, op op) error {
	if op == opAdd {
		var ev epollevent
		ev.data = *(*[8]byte)(unsafe.Pointer(&fd))
		ev.events = syscall.EPOLLHUP | syscall.EPOLLRDHUP | syscall.EPOLLERR | EPOLLET
		return EpollCtl(p.epfd, syscall.EPOLL_CTL_ADD, fd.fd, &ev)
	} else {
		var ev epollevent
		return EpollCtl(p.epfd, syscall.EPOLL_CTL_DEL, fd.fd, &ev)
	}
}

func openpoll() (p poller, err error) {
	var epfd int
	epfd, err = EpollCreate(0)
	if err != nil {
		return nil, err
	}
	return &epoller{epfd: epfd}, nil
}
