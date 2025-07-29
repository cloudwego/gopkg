package connstate

import (
	"errors"
	"fmt"
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

type ConnWithState interface {
	net.Conn
	State() ConnState
}

func ListenConnState(conn net.Conn) (ConnWithState, error) {
	pollInitOnce.Do(func() {
		var err error
		poll, err = openpoll()
		if err != nil {
			panic(fmt.Sprintf("gopkg.connstate openpoll failed, err: %v", err))
		}
	})
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
		fd.conn = unsafe.Pointer(&connWithState{Conn: conn, fd: unsafe.Pointer(fd)})
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
	return (*connWithState)(fd.conn), nil
}

type connWithState struct {
	net.Conn
	fd    unsafe.Pointer // *fdOperator
	state uint32
}

func (c *connWithState) Close() error {
	fd := (*fdOperator)(atomic.LoadPointer(&c.fd))
	if fd != nil && atomic.CompareAndSwapPointer(&c.fd, unsafe.Pointer(fd), nil) {
		atomic.StoreUint32(&c.state, uint32(StateClosed))
		_ = poll.control(fd, opDel)
		pollcache.freeable(fd)
	}
	return c.Conn.Close()
}

func (c *connWithState) State() ConnState {
	return ConnState(atomic.LoadUint32(&c.state))
}
