package iouring

import (
	"sync"
	"unsafe"
)

const UserDataMagic = 0x494E4458494F5552 // "INDXIOUR" - validation magic

var userDataPool = sync.Pool{
	New: func() any {
		return &userData{
			notify: make(chan int32, 1),
		}
	},
}

func userDataPoolGet() *userData {
	u := userDataPool.Get().(*userData)
	u.Reset()
	return u
}

func userDataPoolPut(p *userData) {
	p.magic = 0 // mark as invaild
	userDataPool.Put(p)
}

// userData - tracks in-flight operation
type userData struct {
	magic  uint64
	notify chan int32
	sqe    IOUringSQE
	ivs    []Iovec // for readv / writev
	n      int32
}

func (u *userData) Reset() {
	u.magic = UserDataMagic
	if len(u.notify) > 0 {
		<-u.notify
	}
	// userdata points to self
	u.sqe = IOUringSQE{UserData: uint64(uintptr(unsafe.Pointer(u)))}
	u.n = 0
}

// SetWriteOp configures the SQE for a write operation
//
//go:norace
func (u *userData) SetWriteOp(fd int32, bufs ...[]byte) {
	sqe := &u.sqe
	sqe.Opcode = IORING_OP_WRITEV
	sqe.Fd = fd
	sqe.Off = 0
	sqe.Len = 0
	u.ivs = u.ivs[:0]
	for _, buf := range bufs {
		if len(buf) > 0 {
			u.ivs = append(u.ivs, Iovec{
				Base: uintptr(unsafe.Pointer(&buf[0])),
				Len:  uint64(len(buf)),
			})
		}
	}
	if len(u.ivs) > 0 {
		sqe.Len = uint32(len(u.ivs))
		sqe.Addr = uint64(uintptr(unsafe.Pointer(&u.ivs[0])))
	}
}

// SetReadOp configures the SQE for a read operation
//
//go:norace
func (u *userData) SetReadOp(fd int32, bufs ...[]byte) {
	sqe := &u.sqe
	sqe.Opcode = IORING_OP_READV
	sqe.Fd = fd
	sqe.Off = 0
	sqe.Len = 0
	u.ivs = u.ivs[:0]
	for _, buf := range bufs {
		if len(buf) > 0 {
			u.ivs = append(u.ivs, Iovec{
				Base: uintptr(unsafe.Pointer(&buf[0])),
				Len:  uint64(len(buf)),
			})
		}
	}
	if len(u.ivs) > 0 {
		sqe.Len = uint32(len(u.ivs))
		sqe.Addr = uint64(uintptr(unsafe.Pointer(&u.ivs[0])))
	}
}

//go:nocheckptr
func getUserData(p uint64) *userData {
	return (*userData)(unsafe.Pointer(uintptr(p)))
}

//go:norace
func (u *userData) Copy2SQE(p *IOUringSQE) {
	*p = u.sqe
}

//go:norace
func (u *userData) IsValid() bool {
	return u.magic == UserDataMagic
}

//go:norace
func (u *userData) IsWriteOp() bool {
	return u.sqe.Opcode == IORING_OP_WRITE || u.sqe.Opcode == IORING_OP_WRITEV
}

//go:norace
func (u *userData) AdvanceWrite(n int32) (int32, bool) {
	done := false
	u.n += n // BUG: max 2GB per op

	switch u.sqe.Opcode {
	case IORING_OP_WRITE:
		u.sqe.Addr += uint64(n)
		u.sqe.Len -= uint32(n)
		done = u.sqe.Len == 0

	case IORING_OP_WRITEV:
		wn := uint64(n)
		ivs := u.ivs[:0]
		for i, iv := range u.ivs {
			if iv.Len <= wn {
				wn -= iv.Len
			} else {
				u.ivs[i].Base += uintptr(wn)
				u.ivs[i].Len -= wn
				ivs = append(ivs, u.ivs[i:]...)
				break
			}
		}
		u.ivs = ivs
		done = len(ivs) == 0

	default:
		panic("unexpected type")
	}
	return u.n, done
}

//go:norace
func (u *userData) SendRes(res int32) {
	if u.notify != nil {
		select {
		case u.notify <- res:
		default:
		}
	}
}

func (u *userData) Wait() int32 {
	return <-u.notify
}
