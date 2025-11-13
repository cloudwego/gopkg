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
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestListenConnState(t *testing.T) {
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		panic(err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			assert.Nil(t, err)
			go func(conn net.Conn) {
				buf := make([]byte, 11)
				_, err := conn.Read(buf)
				assert.Nil(t, err)
				conn.Close()
			}(conn)
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

func (m *mockPoller) close() error {
	return nil
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
	if poll != nil {
		_ = poll.close()
	}
	defer func() {
		createPoller()
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
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		panic(err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			assert.Nil(b, err)
			go func(conn net.Conn) {
				buf := make([]byte, 11)
				_, err := conn.Read(buf)
				assert.Nil(b, err)
				conn.Close()
			}(conn)
		}
	}()
	b.ResetTimer()
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

type statefulConn struct {
	net.Conn
	stater ConnStater
}

func (s *statefulConn) Close() error {
	s.stater.Close()
	return s.Conn.Close()
}

type mockStater struct {
}

func (m *mockStater) State() ConnState {
	return StateOK
}

func (m *mockStater) Close() error {
	return nil
}

type connpool struct {
	mu    sync.Mutex
	conns []*statefulConn
}

func (p *connpool) get(dialFunc func() *statefulConn) *statefulConn {
	p.mu.Lock()
	if len(p.conns) == 0 {
		p.mu.Unlock()
		return dialFunc()
	}
	for i := len(p.conns) - 1; i >= 0; i-- {
		conn := p.conns[i]
		if conn.stater.State() == StateOK {
			p.conns = p.conns[:i]
			p.mu.Unlock()
			return conn
		} else {
			conn.Close()
		}
	}
	p.conns = p.conns[:0]
	p.mu.Unlock()
	return dialFunc()
}

func (p *connpool) put(conn *statefulConn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.conns = append(p.conns, conn)
}

func (p *connpool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, conn := range p.conns {
		conn.Close()
	}
	p.conns = p.conns[:0]
	return nil
}

var withListenConnState bool

// BenchmarkWithConnState is used to verify the impact of adding ConnState logic on performance.
// To compare with syscall.EpollWait(), you could run `go test -bench=BenchmarkWith -benchtime=10s .`
// to test the first time, and replace isyscall.EpollWait() with syscall.EpollWait() to test the second time.
func BenchmarkWithConnState(b *testing.B) {
	withListenConnState = true
	benchmarkConnState(b)
}

func BenchmarkWithoutConnState(b *testing.B) {
	withListenConnState = false
	benchmarkConnState(b)
}

func benchmarkConnState(b *testing.B) {
	// set GOMAXPROCS to 1 to make P resources scarce
	runtime.GOMAXPROCS(1)
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		panic(err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			assert.Nil(b, err)
			go func(conn net.Conn) {
				var count uint64
				for {
					buf := make([]byte, 11)
					_, err := conn.Read(buf)
					if err != nil {
						conn.Close()
						return
					}
					_, err = conn.Write(buf)
					if err != nil {
						conn.Close()
						return
					}
					count++
					if count == 1000 {
						conn.Close()
						return
					}
				}
			}(conn)
		}
	}()
	cp := &connpool{}
	dialFunc := func() *statefulConn {
		conn, err := net.Dial("tcp", ln.Addr().String())
		assert.Nil(b, err)
		var stater ConnStater
		if withListenConnState {
			stater, err = ListenConnState(conn)
			assert.Nil(b, err)
		} else {
			stater = &mockStater{}
		}
		return &statefulConn{
			Conn:   conn,
			stater: stater,
		}
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			conn := cp.get(dialFunc)
			buf := make([]byte, 11)
			_, err := conn.Write(buf)
			if err != nil {
				conn.Close()
				continue
			}
			_, err = conn.Read(buf)
			if err != nil {
				conn.Close()
				continue
			}
			cp.put(conn)
		}
	})
	_ = cp.Close()
}
