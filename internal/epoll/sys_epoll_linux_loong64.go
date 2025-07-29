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

package netpoll

import (
	"syscall"
	"unsafe"
)

var _zero uintptr

// EpollWait implements epoll_wait.
func EpollWait(epfd int, events []syscall.EpollEvent, msec int) (n int, err error) {
	var _p0 unsafe.Pointer
	if len(events) > 0 {
		_p0 = unsafe.Pointer(&events[0])
	} else {
		_p0 = unsafe.Pointer(&_zero)
	}
	entersyscallblock()
	r0, _, e1 := syscall.RawSyscall6(syscall.SYS_EPOLL_PWAIT, uintptr(epfd), uintptr(_p0), uintptr(len(events)), uintptr(msec), 0, 0)
	exitsyscall()
	n = int(r0)
	if e1 != 0 {
		err = e1
	}
	return
}
