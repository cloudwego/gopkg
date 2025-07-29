package connstate

import (
	"sync/atomic"
	"syscall"
	"unsafe"
)

const _EPOLLET uint32 = 0x80000000

type epoller struct {
	epfd int
}

func (p *epoller) wait() error {
	events := make([]syscall.EpollEvent, 128)
	for {
		// TODO: handoff p by entersyscallblock, or make poller run as a thread.
		n, err := syscall.EpollWait(p.epfd, events, -1)
		if err != nil && err != syscall.EINTR {
			return err
		}
		for i := 0; i < n; i++ {
			ev := &events[i]
			op := *(**fdOperator)(unsafe.Pointer(&ev.Fd))
			if conn := (*connStater)(atomic.LoadPointer(&op.conn)); conn != nil {
				if ev.Events&(syscall.EPOLLHUP|syscall.EPOLLRDHUP|syscall.EPOLLERR) != 0 {
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
		var ev syscall.EpollEvent
		*(**fdOperator)(unsafe.Pointer(&ev.Fd)) = fd
		ev.Events = syscall.EPOLLHUP | syscall.EPOLLRDHUP | syscall.EPOLLERR | _EPOLLET
		return syscall.EpollCtl(p.epfd, syscall.EPOLL_CTL_ADD, fd.fd, &ev)
	} else {
		var ev syscall.EpollEvent
		return syscall.EpollCtl(p.epfd, syscall.EPOLL_CTL_DEL, fd.fd, &ev)
	}
}

func openpoll() (p poller, err error) {
	var epfd int
	epfd, err = syscall.EpollCreate(1)
	if err != nil {
		return nil, err
	}
	return &epoller{epfd: epfd}, nil
}
