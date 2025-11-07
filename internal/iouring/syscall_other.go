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

//go:build !linux

package iouring

import (
	"syscall"
	"unsafe"
)

// Setup is a stub implementation for non-Linux platforms.
// Returns ENOSYS as io_uring is only supported on Linux.
func Setup(entries uint32, params *IOUringParams) (int, error) {
	return 0, syscall.ENOSYS
}

// Enter is a stub implementation for non-Linux platforms.
// Returns ENOSYS as io_uring is only supported on Linux.
func Enter(fd int, toSubmit uint32, minComplete uint32, flags uint32, sig unsafe.Pointer) (int, syscall.Errno) {
	return 0, syscall.ENOSYS
}

// Register is a stub implementation for non-Linux platforms.
// Returns ENOSYS as io_uring is only supported on Linux.
func Register(fd int, opcode uint32, arg unsafe.Pointer, nrArgs uint32) syscall.Errno {
	return syscall.ENOSYS
}
