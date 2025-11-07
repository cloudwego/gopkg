package iouring

import (
	"sync"
	"time"
)

// ring represents a single io_uring instance with its submission channel
type ring struct {
	r       *IOUring
	sqeChan chan *userData
	mu      sync.Mutex
}

// IOUringEventLoop manages a single io_uring instance for all connections
type IOUringEventLoop struct {
	ring *ring
}

func NewIOUringEventLoop(cfg *Config) (*IOUringEventLoop, error) {
	// Create single io_uring instance
	r, err := NewIOUring(2 * cfg.IOUringQueueSize)
	if err != nil {
		return nil, err
	}

	evl := &IOUringEventLoop{
		ring: &ring{
			r:       r,
			sqeChan: make(chan *userData, cfg.IOUringQueueSize),
		},
	}

	// Start goroutines for the single ring
	go evl.ring.sqeEventLoop(cfg.SQEBatchSize, cfg.SQESubmitInterval)
	go evl.ring.eventLoop()

	return evl, nil
}

func (r *ring) prepareSQE(x *userData) {
	sqe := r.r.PeekSQE(false)
	x.Copy2SQE(sqe)
	r.r.AdvanceSQ()
}

func (r *ring) Submit() {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, errno := r.r.Submit()
	if errno != 0 {
		panic(errno.Error())
	}
}

func (r *ring) SubmitBatch(xx []*userData) {
	for _, x := range xx {
		r.prepareSQE(x)
	}
	r.Submit()
}

// sqeEventLoop - serialize SQE submissions and batch for efficiency
func (r *ring) sqeEventLoop(batchSize int, submitInterval time.Duration) {
	var submitc <-chan time.Time
	if submitInterval > 0 {
		ticker := time.NewTicker(submitInterval)
		defer ticker.Stop()
		submitc = ticker.C
	}
	n := 0
	for {
		select {
		case x, ok := <-r.sqeChan:
			if !ok {
				return
			}
			r.prepareSQE(x)
			n++
		case <-submitc:
			r.Submit()
			n = 0
		}
		if n >= batchSize {
			r.Submit()
			n = 0
		}
	}
}

// eventLoop - wait for completions and dispatch results
func (r *ring) eventLoop() {
	for {
		cqe, err := r.r.WaitCQE()
		if err != nil {
			panic(err)
		}
		// UserData can be 0 for timeout operations
		if cqe.UserData != 0 {
			r.handleUserData(getUserData(cqe.UserData), cqe.Res)
		}
		r.r.AdvanceCQ()
	}
}

func (r *ring) handleUserData(ud *userData, res int32) {
	if !ud.IsValid() {
		return
	}
	if res > 0 && ud.IsWriteOp() {
		n, done := ud.AdvanceWrite(res)
		if !done {
			r.sqeChan <- ud // continue write until its done
			return
		}
		res = n
	}
	ud.SendRes(res) // done, notify user
}
