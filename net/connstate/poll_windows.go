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
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

const (
	windowsPollInterval = 10 * time.Millisecond
	windowsPollErr      = 0x0001
	windowsPollHUP      = 0x0002
	windowsPollRDBand   = 0x0200
)

var windowsWSAPoll = syscall.NewLazyDLL("ws2_32.dll").NewProc("WSAPoll")

type windowsPollFD struct {
	fd      uintptr
	events  int16
	revents int16
}

type windowsPoller struct {
	mu        sync.Mutex
	operators map[*fdOperator]struct{}
}

func (p *windowsPoller) wait() error {
	var operators []*fdOperator
	var events []windowsPollFD
	for {
		operators = operators[:0]
		events = events[:0]

		p.mu.Lock()
		for operator := range p.operators {
			operators = append(operators, operator)
			events = append(events, windowsPollFD{
				fd: uintptr(operator.fd),
				// Request only out-of-band readability. Winsock always reports
				// error conditions, including POLLHUP, while ordinary unread
				// application data must not make this poller spin.
				events: windowsPollRDBand,
			})
		}
		var result uintptr
		var callErr error
		if len(events) > 0 {
			result, _, callErr = windowsWSAPoll.Call(
				uintptr(unsafe.Pointer(&events[0])),
				uintptr(len(events)),
				0,
			)
		}
		p.mu.Unlock()
		runtime.KeepAlive(events)
		if int32(result) == -1 {
			return callErr
		}

		var callbacks []OnRemoteClosed
		if result > 0 {
			for i := range events {
				if events[i].revents&(windowsPollHUP|windowsPollErr) == 0 {
					continue
				}
				operator := operators[i]
				if conn := (*connStater)(atomic.LoadPointer(&operator.conn)); conn != nil {
					if atomic.CompareAndSwapUint32(&conn.state, uint32(StateOK), uint32(StateRemoteClosed)) {
						if conn.onRemoteClosed != nil {
							callbacks = append(callbacks, conn.onRemoteClosed)
						}
					}
				}
			}
		}
		if len(callbacks) > 0 {
			go func(callbacks []OnRemoteClosed) {
				for _, callback := range callbacks {
					callback()
				}
			}(callbacks)
		}
		pollcache.free()
		time.Sleep(windowsPollInterval)
	}
}

func (p *windowsPoller) control(fd *fdOperator, op op) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if op == opAdd {
		p.operators[fd] = struct{}{}
	} else {
		delete(p.operators, fd)
	}
	return nil
}

func openpoll() (p poller, err error) {
	if err := windowsWSAPoll.Find(); err != nil {
		return nil, err
	}
	return &windowsPoller{operators: make(map[*fdOperator]struct{})}, nil
}
