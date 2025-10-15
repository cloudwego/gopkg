/*
 * Copyright 2025 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

//go:build linux && (mips64 || mips64le)

package iouring

import (
	"syscall"
	"unsafe"
)

// Setup initializes io_uring
// Creates an io_uring instance with specified number of entries
// Returns file descriptor on success, error on failure
func Setup(entries uint32, params *IoUringParams) (int, error) {
	fd, _, errno := syscall.Syscall(
		5425, // SYS_IO_URING_SETUP
		uintptr(entries),
		uintptr(unsafe.Pointer(params)),
		0,
	)
	if errno != 0 {
		return -1, errno
	}
	return int(fd), nil
}

// Enter submits and waits for completions
// toSubmit: number of SQEs to submit from submission queue
// minComplete: minimum number of completions to wait for
// flags: IORING_ENTER_* flags
// Returns number of completions available
func Enter(fd int, toSubmit uint32, minComplete uint32, flags uint32, sig unsafe.Pointer) (int, syscall.Errno) {
	r, _, errno := syscall.Syscall6(
		5426, // SYS_IO_URING_ENTER
		uintptr(fd),
		uintptr(toSubmit),
		uintptr(minComplete),
		uintptr(flags),
		uintptr(sig),
		0,
	)
	return int(r), errno
}

// Register registers resources with io_uring (files, buffers, etc.)
// fd: io_uring file descriptor
// opcode: IORING_REGISTER_* operation
// arg: pointer to operation-specific data
// nrArgs: number of items being registered
// Returns 0 on success, errno on failure
func Register(fd int, opcode uint32, arg unsafe.Pointer, nrArgs uint32) syscall.Errno {
	_, _, errno := syscall.Syscall6(
		5427, // SYS_IO_URING_REGISTER
		uintptr(fd),
		uintptr(opcode),
		uintptr(arg),
		uintptr(nrArgs),
		0,
		0,
	)
	return errno
}
