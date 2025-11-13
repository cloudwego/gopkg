// Copyright 2025 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package connstate

/*
#cgo CFLAGS: -O2 -Wall

#include <stdint.h>
#include <stdlib.h>
#include <stdatomic.h>

// CGO function declarations
int cgo_epoll_wait_loop(int epfd, atomic_int_least32_t* freeack_ptr);

*/
import "C"

import (
	"syscall"
	"unsafe"
)

const _EPOLLET uint32 = 0x80000000

type epoller struct {
	epfd int
}

//go:nocheckptr
func (p *epoller) wait() error {
	freeackPtr := (*C.atomic_int_least32_t)(unsafe.Pointer(&pollcache.freeack))
	C.cgo_epoll_wait_loop(C.int(p.epfd), freeackPtr)
	return nil
}

func (p *epoller) control(fd *fdOperator, op op) error {
	if op == opAdd {
		var ev syscall.EpollEvent
		// Pass the address of fd.conn field to avoid struct alignment and GC issues
		*(*unsafe.Pointer)(unsafe.Pointer(&ev.Fd)) = unsafe.Pointer(&fd.conn)
		ev.Events = syscall.EPOLLHUP | syscall.EPOLLRDHUP | syscall.EPOLLERR | _EPOLLET
		return syscall.EpollCtl(p.epfd, syscall.EPOLL_CTL_ADD, fd.fd, &ev)
	} else {
		var ev syscall.EpollEvent
		return syscall.EpollCtl(p.epfd, syscall.EPOLL_CTL_DEL, fd.fd, &ev)
	}
}

func (p *epoller) close() error {
	return syscall.Close(p.epfd)
}

func openpoll() (p poller, err error) {
	var epfd int
	epfd, err = syscall.EpollCreate(1)
	if err != nil {
		return nil, err
	}
	return &epoller{epfd: epfd}, nil
}
