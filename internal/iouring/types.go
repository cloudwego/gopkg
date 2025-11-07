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

package iouring

import "unsafe"

// io_uring_sqe represents a submission queue entry
// This structure describes an I/O operation to be performed
// Size must be exactly 64 bytes for kernel ABI compatibility
type IOUringSQE struct {
	Opcode      uint8     // Operation code (IORING_OP_*)
	Flags       uint8     // Flags modifier for operation
	IoPrio      uint16    // Priority for this request
	Fd          int32     // File descriptor to operate on
	Off         uint64    // Offset for operations (or accept flags)
	Addr        uint64    // Pointer to buffer or input args
	Len         uint32    // Length of buffer or number of iovecs
	OpcodeFlags uint32    // Opcode-specific flags
	UserData    uint64    // User data (returned in CQE)
	BufIndex    uint16    // Index into registered buffer array
	Personality uint16    // Personality to use (registered credentials)
	SpliceFdIn  int32     // File descriptor for splice operations
	_           [2]uint64 // Padding to 64 bytes
}

// io_uring_cqe represents a completion queue entry
// This structure contains the result of a completed I/O operation
// Size must be exactly 16 bytes for kernel ABI compatibility
type IOUringCQE struct {
	UserData uint64 // User data from submission (identifies request)
	Res      int32  // Result of operation (bytes transferred or -errno)
	Flags    uint32 // Flags about the completion
}

// Iovec represents an I/O vector for readv/writev operations
type Iovec struct {
	Base uintptr // Pointer to buffer
	Len  uint64  // Length of buffer
}

// Set updates Iovec by `[]byte`
func (p *Iovec) Set(b []byte) {
	p.Len = uint64(len(b))
	if p.Len > 0 {
		p.Base = uintptr(unsafe.Pointer(&b[0]))
	}
}

// TimeSpec represents a kernel timespec structure for io_uring operations.
// This is used for timeout operations and matches the kernel's __kernel_timespec layout.
type TimeSpec struct {
	TvSec  int64 // Seconds
	TvNsec int64 // Nanoseconds
}

// IsZero returns true if the timespec represents zero time.
func (p *TimeSpec) IsZero() bool {
	return *p == TimeSpec{}
}

// Msghdr represents a message header for sendmsg/recvmsg operations
type Msghdr struct {
	Name       *byte  // Socket address
	Namelen    uint32 // Size of socket address
	_          uint32 // Padding
	Iov        *Iovec // Scatter/gather array
	Iovlen     uint64 // Number of elements in iov
	Control    *byte  // Ancillary data
	Controllen uint64 // Ancillary data buffer length
	Flags      int32  // Flags on received message
	_          int32  // Padding
}
