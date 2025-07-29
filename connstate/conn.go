package connstate

import (
	"errors"
	"net"
	"sync/atomic"
	"syscall"
	"unsafe"
)

type ConnState uint32

const (
	StateOK ConnState = iota
	StateRemoteClosed
	StateClosed
)

type ConnStater interface {
	Close() error
	State() ConnState
}

func ListenConnState(conn net.Conn) (ConnStater, error) {
	pollInitOnce.Do(createPoller)
	sysConn, ok := conn.(syscall.Conn)
	if !ok {
		return nil, errors.New("conn is not syscall.Conn")
	}
	rawConn, err := sysConn.SyscallConn()
	if err != nil {
		return nil, err
	}
	var fd *fdOperator
	var opAddErr error
	err = rawConn.Control(func(fileDescriptor uintptr) {
		fd = pollcache.alloc()
		fd.fd = int(fileDescriptor)
		fd.conn = unsafe.Pointer(&connStater{fd: unsafe.Pointer(fd)})
		opAddErr = poll.control(fd, opAdd)
	})
	if fd != nil {
		if err != nil && opAddErr == nil {
			_ = poll.control(fd, opDel)
		}
		if err != nil || opAddErr != nil {
			pollcache.freeable(fd)
		}
	}
	if err != nil {
		return nil, err
	}
	if opAddErr != nil {
		return nil, opAddErr
	}
	return (*connStater)(fd.conn), nil
}

type connStater struct {
	fd    unsafe.Pointer // *fdOperator
	state uint32
}

func (c *connStater) Close() error {
	fd := (*fdOperator)(atomic.LoadPointer(&c.fd))
	if fd != nil && atomic.CompareAndSwapPointer(&c.fd, unsafe.Pointer(fd), nil) {
		atomic.StoreUint32(&c.state, uint32(StateClosed))
		_ = poll.control(fd, opDel)
		pollcache.freeable(fd)
	}
	return nil
}

func (c *connStater) State() ConnState {
	return ConnState(atomic.LoadUint32(&c.state))
}
