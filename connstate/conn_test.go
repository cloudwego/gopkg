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

import (
	"errors"
	"io"
	"net"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var testMutex sync.Mutex

func TestListenConnState(t *testing.T) {
	testMutex.Lock()
	defer testMutex.Unlock()
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		panic(err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			assert.Nil(t, err)
			go func() {
				buf := make([]byte, 11)
				_, err := conn.Read(buf)
				assert.Nil(t, err)
				conn.Close()
			}()
		}
	}()
	conn, err := net.Dial("tcp", ln.Addr().String())
	assert.Nil(t, err)
	stater, err := ListenConnState(conn)
	assert.Nil(t, err)
	assert.Equal(t, StateOK, stater.State())
	_, err = conn.Write([]byte("hello world"))
	assert.Nil(t, err)
	buf := make([]byte, 1)
	_, err = conn.Read(buf)
	assert.Equal(t, io.EOF, err)
	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, StateRemoteClosed, stater.State())
	assert.Nil(t, stater.Close())
	assert.Nil(t, conn.Close())
	assert.Equal(t, StateClosed, stater.State())
}

type mockPoller struct {
	controlFunc func(fd *fdOperator, op op) error
}

func (m *mockPoller) wait() error {
	return nil
}

func (m *mockPoller) control(fd *fdOperator, op op) error {
	return m.controlFunc(fd, op)
}

type mockConn struct {
	net.Conn
	controlFunc func(f func(fd uintptr)) error
}

func (c *mockConn) SyscallConn() (syscall.RawConn, error) {
	return &mockRawConn{
		controlFunc: c.controlFunc,
	}, nil
}

type mockRawConn struct {
	syscall.RawConn
	controlFunc func(f func(fd uintptr)) error
}

func (r *mockRawConn) Control(f func(fd uintptr)) error {
	return r.controlFunc(f)
}

func TestListenConnState_Err(t *testing.T) {
	testMutex.Lock()
	defer testMutex.Unlock()
	// replace poll
	pollInitOnce.Do(createPoller)
	oldPoll := poll
	defer func() {
		poll = oldPoll
	}()
	// test detach
	var expectDetach bool
	defer func() {
		assert.True(t, expectDetach)
	}()
	cases := []struct {
		name            string
		connControlFunc func(f func(fd uintptr)) error
		pollControlFunc func(fd *fdOperator, op op) error
		expectErr       error
	}{
		{
			name: "err conn control",
			connControlFunc: func(f func(fd uintptr)) error {
				return errors.New("err conn control")
			},
			expectErr: errors.New("err conn control"),
		},
		{
			name: "err poll control",
			connControlFunc: func(f func(fd uintptr)) error {
				f(1)
				return nil
			},
			pollControlFunc: func(fd *fdOperator, op op) error {
				assert.Equal(t, fd.fd, 1)
				return errors.New("err poll control")
			},
			expectErr: errors.New("err poll control"),
		},
		{
			name: "err conn control after poll add",
			connControlFunc: func(f func(fd uintptr)) error {
				f(1)
				return errors.New("err conn control after poll add")
			},
			pollControlFunc: func(fd *fdOperator, op op) error {
				if op == opDel {
					expectDetach = true
				}
				assert.Equal(t, fd.fd, 1)
				return nil
			},
			expectErr: errors.New("err conn control after poll add"),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			poll = &mockPoller{
				controlFunc: c.pollControlFunc,
			}
			conn := &mockConn{
				controlFunc: c.connControlFunc,
			}
			_, err := ListenConnState(conn)
			assert.Equal(t, c.expectErr, err)
		})
	}
}

func BenchmarkListenConnState(b *testing.B) {
	testMutex.Lock()
	defer testMutex.Unlock()
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		panic(err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			assert.Nil(b, err)
			go func() {
				buf := make([]byte, 11)
				_, err := conn.Read(buf)
				assert.Nil(b, err)
				conn.Close()
			}()
		}
	}()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			conn, err := net.Dial("tcp", ln.Addr().String())
			assert.Nil(b, err)
			stater, err := ListenConnState(conn)
			assert.Nil(b, err)
			assert.Equal(b, StateOK, stater.State())
			_, err = conn.Write([]byte("hello world"))
			assert.Nil(b, err)
			buf := make([]byte, 1)
			_, err = conn.Read(buf)
			assert.Equal(b, io.EOF, err)
			time.Sleep(10 * time.Millisecond)
			assert.Equal(b, StateRemoteClosed, stater.State())
			assert.Nil(b, stater.Close())
			assert.Nil(b, conn.Close())
			assert.Equal(b, StateClosed, stater.State())
		}
	})
}
