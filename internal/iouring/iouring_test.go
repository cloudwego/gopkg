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

import (
	"net"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// skipIfUnsupported checks if io_uring is available and skips the test if not
func skipIfUnsupported(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("io_uring is only supported on Linux")
	}

	// Try to create a minimal io_uring to check kernel support
	ring, err := NewIOUring(2)
	require.NoError(t, err)
	ring.Close()
}

// getFd extracts the file descriptor from a net.Conn
func getFd(t *testing.T, conn net.Conn) int {
	t.Helper()

	syscallConn, err := conn.(syscall.Conn).SyscallConn()
	require.NoError(t, err)

	var fd int
	err = syscallConn.Control(func(f uintptr) {
		fd = int(f)
	})
	require.NoError(t, err)

	return fd
}

// connPair represents a client-server connection pair
type connPair struct {
	client net.Conn
	server net.Conn
}

func (p *connPair) Close() {
	_ = p.client.Close()
	_ = p.server.Close()
}

// createConnections creates n TCP connection pairs
func createConnections(t *testing.T, n int) []connPair {
	t.Helper()

	// Create a listener
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	ret := make([]connPair, n)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() { // Accept server connections
		defer wg.Done()
		for i := 0; i < n; i++ {
			conn, err := ln.Accept()
			require.NoError(t, err)
			ret[i].server = conn
		}
	}()
	addr := ln.Addr().String()
	for i := 0; i < n; i++ { // Create client connection
		conn, err := net.Dial("tcp", addr)
		require.NoError(t, err)
		ret[i].client = conn
	}
	wg.Wait()
	return ret
}

func TestConnectionReadWrite(t *testing.T) {
	skipIfUnsupported(t)

	ring, err := NewIOUring(10)
	require.NoError(t, err)
	defer ring.Close()

	c := createConnections(t, 1)[0]
	defer c.Close()

	// Prepare read buffer for server
	readBuf := make([]byte, 128)
	readIov := Iovec{
		Base: uintptr(unsafe.Pointer(&readBuf[0])),
		Len:  uint64(len(readBuf)),
	}

	// Submit READV operation on server side
	sqe := ring.PeekSQE(true)
	sqe.Opcode = IORING_OP_READV
	sqe.Fd = int32(getFd(t, c.server))
	sqe.Addr = uint64(uintptr(unsafe.Pointer(&readIov)))
	sqe.Len = 1 // number of iovecs
	sqe.UserData = 100
	ring.AdvanceSQ()

	// Prepare write buffer for client
	testData := []byte("hello world")
	writeIov := [3]Iovec{}
	writeIov[0].Set(testData[0:6])
	writeIov[1].Set(testData[6:7])
	writeIov[2].Set(testData[7:])

	// Submit WRITEV operation on client side
	sqe = ring.PeekSQE(true)
	sqe.Opcode = IORING_OP_WRITEV
	sqe.Fd = int32(getFd(t, c.client))
	sqe.Addr = uint64(uintptr(unsafe.Pointer(&writeIov[0])))
	sqe.Len = 3 // number of iovecs
	sqe.UserData = 200
	ring.AdvanceSQ()

	// Submit both operations to kernel
	submitted, errno := ring.Submit()
	require.Equal(t, syscall.Errno(0), errno)
	require.Equal(t, 2, submitted)

	// Wait for both completions
	var readRes, writeRes int32
	for i := 0; i < 2; i++ {
		cqe, err := ring.WaitCQE()
		require.NoError(t, err)

		switch cqe.UserData {
		case 100: // read completion
			require.GreaterOrEqual(t, cqe.Res, int32(0))
			readRes = cqe.Res
			t.Logf("read completed: %d bytes", cqe.Res)
		case 200: // write completion
			require.GreaterOrEqual(t, cqe.Res, int32(0))
			writeRes = cqe.Res
			t.Logf("write completed: %d bytes", cqe.Res)
		default:
			require.Fail(t, "unexpected user data")
		}
		ring.AdvanceCQ()
	}

	// Verify write completed successfully
	require.Equal(t, int32(len(testData)), writeRes)

	// Verify read completed successfully
	require.Equal(t, int32(len(testData)), readRes)

	// Verify data
	readData := readBuf[:readRes]
	assert.Equal(t, string(testData), string(readData))

	t.Logf("successfully transferred %d bytes: %q", readRes, readData)
}

func TestConnectionClosed(t *testing.T) {
	skipIfUnsupported(t)

	const numConns = 10

	// Create io_uring instance with enough entries
	ring, err := NewIOUring(2 * numConns)
	require.NoError(t, err)
	defer ring.Close()

	// Create 10 connections
	conns := createConnections(t, numConns)
	defer func() {
		for _, p := range conns {
			p.Close()
		}
	}()

	// Submit POLL_ADD operations for all server connections
	for i := 0; i < numConns; i++ {
		sqe := ring.PeekSQE(true)
		require.NotNil(t, sqe)
		sqe.Opcode = IORING_OP_POLL_ADD
		sqe.Fd = int32(getFd(t, conns[i].server))
		sqe.UserData = uint64(i) // Use index as user data
		sqe.OpcodeFlags = uint32(POLLHUP | POLLERR | POLLRDHUP)
		ring.AdvanceSQ()
	}
	// Submit all operations to kernel
	submitted, errno := ring.Submit()
	require.Equal(t, syscall.Errno(0), errno)
	require.Equal(t, numConns, submitted)

	// Close some connections
	closedIndices := make(map[int]bool)
	for _, i := range []int{1, 4, 7} {
		conns[i].client.Close()
		closedIndices[i] = true
	}

	// Wait for completions from the closed connections
	time.Sleep(10 * time.Millisecond) // 10ms should be enough
	for i := 0; i < len(closedIndices); i++ {
		cqe := ring.PeekCQE() // using PeekCQE without calling io_uring_enter
		require.NotNil(t, cqe)
		assert.True(t, closedIndices[int(cqe.UserData)])
		assert.NotZero(t, uint32(cqe.Res)&(POLLHUP|POLLRDHUP|POLLERR))
		t.Logf("conn closed: %d", cqe.UserData)
		ring.AdvanceCQ() // next CQE
	}
}
