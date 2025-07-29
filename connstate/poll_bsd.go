//go:build darwin || netbsd || freebsd || openbsd || dragonfly
// +build darwin netbsd freebsd openbsd dragonfly

package connstate

import (
	"sync/atomic"
	"syscall"
	"unsafe"
)

type kqueue struct {
	fd int
}

func (p *kqueue) wait() error {
	events := make([]syscall.Kevent_t, 1024)
	for {
		// TODO: handoff p by entersyscallblock, or make poller run as a thread.
		n, err := syscall.Kevent(p.fd, nil, events, nil)
		if err != nil && err != syscall.EINTR {
			// exit gracefully
			if err == syscall.EBADF {
				return nil
			}
			return err
		}
		for i := 0; i < n; i++ {
			ev := &events[i]
			op := *(**fdOperator)(unsafe.Pointer(&ev.Udata))
			if conn := (*connWithState)(atomic.LoadPointer(&op.conn)); conn != nil {
				if ev.Flags&(syscall.EV_EOF) != 0 {
					atomic.CompareAndSwapUint32(&conn.state, uint32(StateOK), uint32(StateRemoteClosed))
				}
			}
		}
		// we can make sure that there is no op remaining if finished handling all events
		pollcache.free()
	}
}

func (p *kqueue) control(fd *fdOperator, op op) error {
	evs := make([]syscall.Kevent_t, 1)
	evs[0].Ident = uint64(fd.fd)
	*(**fdOperator)(unsafe.Pointer(&evs[0].Udata)) = fd
	if op == opAdd {
		evs[0].Filter = syscall.EVFILT_READ
		evs[0].Flags = syscall.EV_ADD | syscall.EV_ENABLE | syscall.EV_CLEAR
		// prevent ordinary data from triggering
		evs[0].Flags |= syscall.EV_OOBAND
		evs[0].Fflags = syscall.NOTE_LOWAT
		evs[0].Data = 0x7FFFFFFF
		_, err := syscall.Kevent(p.fd, evs, nil, nil)
		return err
	} else {
		evs[0].Filter = syscall.EVFILT_READ
		evs[0].Flags = syscall.EV_DELETE
		_, err := syscall.Kevent(p.fd, evs, nil, nil)
		return err
	}
}

func openpoll() (p poller, err error) {
	fd, err := syscall.Kqueue()
	if err != nil {
		return nil, err
	}
	_, err = syscall.Kevent(fd, []syscall.Kevent_t{{
		Ident:  0,
		Filter: syscall.EVFILT_USER,
		Flags:  syscall.EV_ADD | syscall.EV_CLEAR,
	}}, nil, nil)
	if err != nil {
		syscall.Close(fd)
		return nil, err
	}
	return &kqueue{fd: fd}, nil
}
