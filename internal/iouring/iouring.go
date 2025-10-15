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

// Package iouring provides a low-level interface to Linux io_uring for high-performance
// asynchronous I/O operations. io_uring enables efficient submission and completion of I/O
// operations through shared memory ring buffers, avoiding syscall overhead for each operation.
//
// This package implements the core io_uring functionality including:
//   - Ring buffer management (submission queue and completion queue)
//   - Support for various I/O operations (read, write, poll, accept, connect, etc.)
//
// Requires Linux kernel 5.4+ with IORING_FEAT_SINGLE_MMAP support.
//
// Example usage:
//
//	ring, err := iouring.NewIoUring(32)
//	if err != nil {
//	    // handle error
//	}
//	defer ring.Close()
//
//	// Submit an operation
//	sqe := ring.PeekSQE(true)
//	sqe.Opcode = iouring.IORING_OP_NOP
//	ring.AdvanceSQ()
//	ring.Submit()
//
//	// Check for completion without blocking
//	if cqe := ring.PeekCQE(); cqe != nil {
//	    // process result
//	    ring.AdvanceCQ()
//	}
//
//	// Or wait for completion (blocking)
//	cqe, err := ring.WaitCQE()
//	if err != nil {
//	    // handle error
//	}
//	// process result
//	ring.AdvanceCQ()
package iouring

import (
	"fmt"
	"runtime"
	"sync/atomic"
	"syscall"
	"unsafe"
)

// io_uring opcodes - these define the type of I/O operation
// Each operation is submitted via the submission queue
const (
	IORING_OP_NOP             = 0  // No operation (useful for testing)
	IORING_OP_READV           = 1  // Vectored read (readv)
	IORING_OP_WRITEV          = 2  // Vectored write (writev)
	IORING_OP_FSYNC           = 3  // File synchronization
	IORING_OP_READ_FIXED      = 4  // Read using pre-registered buffers
	IORING_OP_WRITE_FIXED     = 5  // Write using pre-registered buffers
	IORING_OP_POLL_ADD        = 6  // Add a poll request
	IORING_OP_POLL_REMOVE     = 7  // Remove a poll request
	IORING_OP_SYNC_FILE_RANGE = 8  // Sync file range
	IORING_OP_SENDMSG         = 9  // Send message on socket
	IORING_OP_RECVMSG         = 10 // Receive message from socket
	IORING_OP_TIMEOUT         = 11 // Timeout operation
	IORING_OP_ACCEPT          = 13 // Accept incoming connection (Linux 5.5+)
	IORING_OP_ASYNC_CANCEL    = 14 // Cancel async operation (Linux 5.5+)
	IORING_OP_LINK_TIMEOUT    = 15 // Linked timeout (Linux 5.5+)
	IORING_OP_CONNECT         = 16 // Connect to socket (Linux 5.5+)
	IORING_OP_READ            = 22 // Read from file descriptor (Linux 5.6+)
	IORING_OP_WRITE           = 23 // Write to file descriptor (Linux 5.6+)
	IORING_OP_SEND            = 26 // Send data on socket (Linux 5.6+)
	IORING_OP_RECV            = 27 // Receive data from socket (Linux 5.6+)
	IORING_OP_CLOSE           = 19 // Close file descriptor (Linux 5.6+)
)

// io_uring setup flags - control behavior of the io_uring instance
const (
	IORING_SETUP_IOPOLL     = (1 << 0) // Perform busy-waiting for I/O completion
	IORING_SETUP_SQPOLL     = (1 << 1) // Use kernel thread for submission queue polling
	IORING_SETUP_SQ_AFF     = (1 << 2) // Set CPU affinity for SQPOLL thread
	IORING_SETUP_CQSIZE     = (1 << 3) // App specifies CQ size (must be power of 2)
	IORING_SETUP_CLAMP      = (1 << 4) // Clamp SQ/CQ ring sizes to kernel limits
	IORING_SETUP_ATTACH_WQ  = (1 << 5) // Attach to existing workqueue
	IORING_SETUP_R_DISABLED = (1 << 6) // Start with ring disabled (Linux 5.10+)
)

// io_uring feature flags - returned in params.Features after setup
const (
	IORING_FEAT_SINGLE_MMAP = (1 << 0) // SQ and CQ rings can be mapped with a single mmap (kernel 5.4+)
)

// io_uring enter flags - control behavior of io_uring_enter syscall
const (
	IORING_ENTER_GETEVENTS = (1 << 0) // Wait for completion events
	IORING_ENTER_SQ_WAKEUP = (1 << 1) // Wake SQPOLL thread if sleeping
	IORING_ENTER_SQ_WAIT   = (1 << 2) // Wait for SQPOLL thread to finish
	IORING_ENTER_EXT_ARG   = (1 << 3) // Pass extended argument (Linux 5.11+)
)

// SQE flags - control behavior of individual operations
const (
	IOSQE_FIXED_FILE = (1 << 0) // Use fixed (registered) file descriptor
	IOSQE_IO_LINK    = (1 << 2) // Link next SQE in chain
)

// io_uring register opcodes - for SYS_IO_URING_REGISTER
const (
	IORING_REGISTER_BUFFERS      = 0 // Register buffers for fixed buffer I/O
	IORING_UNREGISTER_BUFFERS    = 1 // Unregister buffers
	IORING_REGISTER_FILES        = 2 // Register file descriptors
	IORING_UNREGISTER_FILES      = 3 // Unregister file descriptors
	IORING_REGISTER_EVENTFD      = 4 // Register eventfd for completion notifications
	IORING_UNREGISTER_EVENTFD    = 5 // Unregister eventfd
	IORING_REGISTER_FILES_UPDATE = 6 // Update registered files (Linux 5.5+)
)

// Poll event flags - for IORING_OP_POLL_ADD
const (
	POLLIN    = 0x0001 // Data available to read
	POLLOUT   = 0x0004 // Ready for writing
	POLLERR   = 0x0008 // Error condition
	POLLHUP   = 0x0010 // Hang up (peer closed)
	POLLNVAL  = 0x0020 // Invalid request
	POLLRDHUP = 0x2000 // Peer closed or shutdown write half
)

// io_uring_params for setup syscall
// Used both as input (flags, sq_thread_*) and output (features, offsets)
type IoUringParams struct {
	SqEntries    uint32          // Number of submission queue entries (power of 2)
	CqEntries    uint32          // Number of completion queue entries
	Flags        uint32          // Setup flags (IORING_SETUP_*)
	SqThreadCpu  uint32          // CPU for SQPOLL thread
	SqThreadIdle uint32          // Milliseconds before SQPOLL thread sleeps
	Features     uint32          // Kernel-supported features (output)
	WqFd         uint32          // Existing workqueue fd to attach to
	Resv         [3]uint32       // Reserved for future use
	SqOff        IoSqringOffsets // Submission queue ring offsets (output)
	CqOff        IoCqringOffsets // Completion queue ring offsets (output)
}

// IoSqringOffsets - byte offsets into mmap'd SQ ring for locating fields
type IoSqringOffsets struct {
	Head        uint32 // Head pointer (consumer, kernel updates)
	Tail        uint32 // Tail pointer (producer, app updates)
	RingMask    uint32 // Ring mask (entries - 1)
	RingEntries uint32 // Ring size
	Flags       uint32
	Dropped     uint32
	Array       uint32 // SQE index indirection array
	Resv1       uint32
	Resv2       uint64
}

// IoCqringOffsets - byte offsets into mmap'd CQ ring for locating fields
type IoCqringOffsets struct {
	Head        uint32 // Head pointer (consumer, app updates)
	Tail        uint32 // Tail pointer (producer, kernel updates)
	RingMask    uint32 // Ring mask (entries - 1)
	RingEntries uint32 // Ring size
	Overflow    uint32 // Overflow counter
	Cqes        uint32 // CQE array start
	Flags       uint64
	Resv1       uint32
	Resv2       uint64
}

// IoUring represents an io_uring instance
// Contains the file descriptor and memory-mapped regions
type IoUring struct {
	fd      int             // io_uring file descriptor
	params  IoUringParams   // Parameters from setup
	sq      SubmissionQueue // Submission queue state
	cq      CompletionQueue // Completion queue state
	sqeMem  []byte          // Memory-mapped SQE array
	ringMem []byte          // Memory-mapped SQ/CQ ring (single mmap, IORING_FEAT_SINGLE_MMAP)
}

// SubmissionQueue represents the submission queue state.
// The submission queue is used to submit I/O operations to the kernel.
// Application acts as producer (updates tail), kernel acts as consumer (updates head).
type SubmissionQueue struct {
	head        *uint32      // Consumer index (kernel) - shared, modified at runtime
	tail        *uint32      // Producer index (app) - shared, modified at runtime
	ringMask    uint32       // Mask for ring wrap - constant after init
	ringEntries uint32       // Number of entries - constant after init
	flags       *uint32      // Flags - shared, modified at runtime
	dropped     *uint32      // Dropped submissions - shared, modified at runtime
	array       *uint32      // SQE index array - pointer for indexing
	sqes        []IoUringSQE // Submission queue entries array
}

// CompletionQueue represents the completion queue state.
// The completion queue is used to receive I/O operation results from the kernel.
// Kernel acts as producer (updates tail), application acts as consumer (updates head).
type CompletionQueue struct {
	head        *uint32      // Consumer index (app) - shared, modified at runtime
	tail        *uint32      // Producer index (kernel) - shared, modified at runtime
	ringMask    uint32       // Mask for ring wrap - constant after init
	ringEntries uint32       // Number of entries - constant after init
	overflow    *uint32      // Overflow counter - shared, modified at runtime
	cqes        []IoUringCQE // Completion queue entries array
}

// NewIoUring creates a new io_uring instance
// entries: size of submission queue (must be power of 2)
// Returns initialized io_uring instance with memory mappings
// Requires Linux 5.4+ (IORING_FEAT_SINGLE_MMAP support)
func NewIoUring(entries uint32) (*IoUring, error) {
	params := IoUringParams{}
	fd, err := Setup(entries, &params)
	if err != nil {
		return nil, fmt.Errorf("io_uring_setup failed: %v", err)
	}

	// Check for IORING_FEAT_SINGLE_MMAP support (Linux 5.4+)
	if params.Features&IORING_FEAT_SINGLE_MMAP == 0 {
		syscall.Close(fd)
		return nil, fmt.Errorf("kernel does not support IORING_FEAT_SINGLE_MMAP (requires Linux 5.4+)")
	}

	ring := &IoUring{
		fd:     fd,
		params: params,
	}

	pageSize := uint32(syscall.Getpagesize())

	// Use single mmap for both SQ and CQ rings (IORING_FEAT_SINGLE_MMAP)
	// Calculate size to cover both rings - need to include both SQ and CQ regions
	sqRingSize := params.SqOff.Array + params.SqEntries*uint32(unsafe.Sizeof(uint32(0)))
	cqRingSize := params.CqOff.Cqes + params.CqEntries*uint32(unsafe.Sizeof(IoUringCQE{}))

	// Take the maximum of both sizes to ensure we map enough memory
	ringSize := sqRingSize
	if cqRingSize > ringSize {
		ringSize = cqRingSize
	}
	// Ensure page-aligned size
	ringSize = (ringSize + pageSize - 1) &^ (pageSize - 1)

	ringPtr, err := syscall.Mmap(fd, 0, int(ringSize),
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_SHARED|syscall.MAP_POPULATE)
	if err != nil {
		ring.Close()
		return nil, fmt.Errorf("mmap ring (single) failed: %v", err)
	}
	ring.ringMem = ringPtr

	// Map SQE array (separate mapping at offset 0x10000000)
	sqeSize := params.SqEntries * uint32(unsafe.Sizeof(IoUringSQE{}))
	sqePtr, err := syscall.Mmap(fd, int64(0x10000000), int(sqeSize),
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_SHARED|syscall.MAP_POPULATE)
	if err != nil {
		ring.Close()
		return nil, fmt.Errorf("mmap sqe failed: %v", err)
	}
	ring.sqeMem = sqePtr

	// Setup SQ pointers into shared memory (use atomics for head/tail)
	ring.sq.head = (*uint32)(unsafe.Pointer(&ring.ringMem[params.SqOff.Head]))
	ring.sq.tail = (*uint32)(unsafe.Pointer(&ring.ringMem[params.SqOff.Tail]))
	ring.sq.ringMask = *(*uint32)(unsafe.Pointer(&ring.ringMem[params.SqOff.RingMask]))
	ring.sq.ringEntries = *(*uint32)(unsafe.Pointer(&ring.ringMem[params.SqOff.RingEntries]))
	ring.sq.flags = (*uint32)(unsafe.Pointer(&ring.ringMem[params.SqOff.Flags]))
	ring.sq.dropped = (*uint32)(unsafe.Pointer(&ring.ringMem[params.SqOff.Dropped]))
	ring.sq.array = (*uint32)(unsafe.Pointer(&ring.ringMem[params.SqOff.Array]))
	ring.sq.sqes = (*[0x10000]IoUringSQE)(unsafe.Pointer(&ring.sqeMem[0]))[:params.SqEntries]

	// Setup completion queue pointers and values
	// Pointers are shared with kernel - must use atomic operations
	// Constants are read once and stored as values
	ring.cq.head = (*uint32)(unsafe.Pointer(&ring.ringMem[params.CqOff.Head]))
	ring.cq.tail = (*uint32)(unsafe.Pointer(&ring.ringMem[params.CqOff.Tail]))
	ring.cq.ringMask = *(*uint32)(unsafe.Pointer(&ring.ringMem[params.CqOff.RingMask]))
	ring.cq.ringEntries = *(*uint32)(unsafe.Pointer(&ring.ringMem[params.CqOff.RingEntries]))
	ring.cq.overflow = (*uint32)(unsafe.Pointer(&ring.ringMem[params.CqOff.Overflow]))
	cqesPtr := unsafe.Pointer(&ring.ringMem[params.CqOff.Cqes])
	ring.cq.cqes = (*[0x10000]IoUringCQE)(cqesPtr)[:params.CqEntries]

	// Set finalizer to ensure cleanup on GC
	runtime.SetFinalizer(ring, func(r *IoUring) {
		r.Close()
	})

	return ring, nil
}

// PeekSQE gets a submission queue entry for the caller to fill.
// It does NOT make the entry visible to the kernel.
// Returns nil if the submission queue is full.
// After filling the SQE, the caller must call AdvanceSQ() to make it visible.
// The caller is responsible for setting all necessary fields of the SQE,
// as the returned SQE may contain stale data from a previous operation.
func (ring *IoUring) PeekSQE(reset bool) *IoUringSQE {
	q := &ring.sq

	tail := atomic.LoadUint32(q.tail)
	head := atomic.LoadUint32(q.head)

	// Check if queue is full: (tail - head) >= q.ringEntries
	if tail-head >= q.ringEntries {
		return nil
	}

	sqe := &q.sqes[tail&q.ringMask]

	if reset {
		*sqe = IoUringSQE{}
	}

	// Update indirection array: array[ring_pos] = sqe_index.
	// This write is made visible by the memory barrier in AdvanceSQ.
	arrayIdx := tail & q.ringMask
	arrayPtr := (*uint32)(unsafe.Pointer(uintptr(unsafe.Pointer(q.array)) + uintptr(arrayIdx)*4))
	*arrayPtr = arrayIdx

	return sqe
}

// AdvanceSQ makes one submission queue entry visible to the kernel.
// This should be called after the SQE from PeekSQE has been populated.
// This acts as a memory barrier.
func (ring *IoUring) AdvanceSQ() {
	atomic.AddUint32(ring.sq.tail, 1)
}

// PendingSQEs returns the number of submission queue entries that have been
// queued but not yet submitted to the kernel.
func (ring *IoUring) PendingSQEs() uint32 {
	return atomic.LoadUint32(ring.sq.tail) - atomic.LoadUint32(ring.sq.head)
}

// Submit submits queued entries
// Calls io_uring_enter to notify kernel of new submissions
// Returns number of submissions accepted by kernel
func (ring *IoUring) Submit() (int, syscall.Errno) {
	// Number of pending SQEs = tail - head
	toSubmit := ring.PendingSQEs()
	if toSubmit == 0 {
		return 0, 0
	}

	// Submit to kernel, retry on EINTR
	for {
		submitted, errno := Enter(ring.fd, toSubmit, 0, 0, nil)
		if errno == syscall.EINTR {
			continue
		}
		if errno != 0 {
			return submitted, errno
		}
		return submitted, 0
	}
}

// PeekCQE checks for a completion queue entry without blocking
// Returns nil if no completion is available
// Returns the CQE but does NOT advance the head - call AdvanceCQ after processing
func (ring *IoUring) PeekCQE() *IoUringCQE {
	q := &ring.cq
	head := atomic.LoadUint32(q.head)
	tail := atomic.LoadUint32(q.tail)

	// Return nil if queue is empty
	if head == tail {
		return nil
	}

	// Get CQE at head position
	cqe := &q.cqes[head&q.ringMask]
	return cqe
}

// WaitCQE waits for a completion queue entry
// Blocks until at least one completion is available
// Returns the CQE but does NOT advance the head - call AdvanceCQ after processing
func (ring *IoUring) WaitCQE() (*IoUringCQE, error) {
	q := &ring.cq
	// Use atomic loads - kernel is producer, app is consumer for CQ
	head := atomic.LoadUint32(q.head)
	tail := atomic.LoadUint32(q.tail)

	// If queue is empty, wait for completions (retry on EINTR/EAGAIN)
	for head == tail {
		_, errno := Enter(ring.fd, 0, 1, IORING_ENTER_GETEVENTS, nil)
		if errno == syscall.EINTR || errno == syscall.EAGAIN {
			// Small backoff instead of busy waiting
			runtime.Gosched()
			tail = atomic.LoadUint32(q.tail)
			continue
		}
		if errno != 0 {
			return nil, errno
		}
		// Reload after kernel may have produced new entries
		tail = atomic.LoadUint32(q.tail)
	}

	// Get CQE at head position - make a copy to avoid data races
	cqe := &q.cqes[head&q.ringMask]
	return cqe, nil
}

// AdvanceCQ advances the completion queue head by one, freeing the oldest CQE slot.
func (ring *IoUring) AdvanceCQ() {
	atomic.AddUint32(ring.cq.head, 1)
}

// Close closes the io_uring instance and releases all associated resources.
// This includes unregistering files, unmapping memory regions, and closing the file descriptor.
// Returns the first error encountered during cleanup, if any.
func (ring *IoUring) Close() error {
	if ring == nil {
		return nil
	}
	runtime.SetFinalizer(ring, nil)

	var firstErr error

	// Unmap SQ/CQ ring (single mmap, IORING_FEAT_SINGLE_MMAP)
	if ring.ringMem != nil {
		if err := syscall.Munmap(ring.ringMem); err != nil && firstErr == nil {
			firstErr = err
		}
		ring.ringMem = nil
	}

	// Unmap SQE array
	if ring.sqeMem != nil {
		if err := syscall.Munmap(ring.sqeMem); err != nil && firstErr == nil {
			firstErr = err
		}
		ring.sqeMem = nil
	}
	if ring.fd >= 0 {
		if err := syscall.Close(ring.fd); err != nil && firstErr == nil {
			firstErr = err
		}
		ring.fd = -1
	}
	return firstErr
}
