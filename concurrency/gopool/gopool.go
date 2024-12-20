package gopool

import (
	"context"
	"log"
	"runtime/debug"
	"sync/atomic"
	"time"
)

// Option ...
type Option struct {
	MaxIdleWorkers int           // it keeps up to `MaxIdleWorkers` workers in pool
	WorkerMaxAge   time.Duration // workers exit after `WorkerMaxAge`
}

var defaultOption = Option{
	MaxIdleWorkers: 10000,
	WorkerMaxAge:   10 * time.Second,
}

func DefaultOption() *Option {
	o := defaultOption
	return &o
}

var defaultGoPool = NewGoPool("__default__", nil)

// Go runs the given func in background
func Go(f func()) {
	defaultGoPool.Go(f)
}

// GoCtx runs the given func in background
func GoCtx(ctx context.Context, f func()) {
	defaultGoPool.GoCtx(ctx, f)
}

// SetPanicHandler sets a func for handling panic cases.
//
// check the comment of (*GoPool).SetPanicHandler for details
func SetPanicHandler(f func(ctx context.Context, r interface{})) {
	defaultGoPool.SetPanicHandler(f)
}

type task struct {
	ctx context.Context
	f   func()

	typ uint8
}

const (
	noopTask = 1
)

// GoPool represents a simple worker pool which manages goroutines for background tasks.
type GoPool struct {
	name string

	workers int32
	maxIdle int32
	maxage  int64 // milliseconds

	panicHandler func(ctx context.Context, r interface{})

	tasks     chan task
	unixMilli int64

	createWorker func()
}

// this is just a magic number of task queue length.
// if it's full, we will fall back to use `go` directly without using pool
// normally, the queue length should be small,
// coz we will create a new worker to pick it as soon as possible.
const defaultChanBufferSize = 1000

// NewGoPool create a new instance for goroutine worker
func NewGoPool(name string, o *Option) *GoPool {
	if o == nil {
		o = DefaultOption()
	}
	p := &GoPool{
		name:    name,
		tasks:   make(chan task, defaultChanBufferSize),
		maxage:  o.WorkerMaxAge.Milliseconds(),
		maxIdle: int32(o.MaxIdleWorkers),
	}

	// fix: func literal escapes to heap
	p.createWorker = func() {
		p.runWorker()
	}
	return p
}

// Go runs the given func in background
func (p *GoPool) Go(f func()) {
	p.GoCtx(context.Background(), f)
}

// GoCtx runs the given func in background
func (p *GoPool) GoCtx(ctx context.Context, f func()) {
	select {
	case p.tasks <- task{ctx: ctx, f: f}:
	default:
		// full? fall back to use go directly
		go p.runTask(ctx, f)
	}
	// luckily ... it's true when there're many workers.
	if len(p.tasks) == 0 {
		return
	}
	// all worker is busy, create a new one
	go p.createWorker()
}

// SetPanicHandler sets a func for handling panic cases.
//
// Panic handler takes two args, `ctx` and `r`.
// `ctx` is the one provided when calling GoCtx, and `r` is returned by recover()
//
// By default, GoPool will use log.Printf to record the err and stack.
//
// It's recommended to set your own handler.
func (p *GoPool) SetPanicHandler(f func(ctx context.Context, r interface{})) {
	p.panicHandler = f
}

func (p *GoPool) runTask(ctx context.Context, f func()) {
	defer func(p *GoPool, ctx context.Context) {
		if r := recover(); r != nil {
			if p.panicHandler != nil {
				p.panicHandler(ctx, r)
			} else {
				log.Printf("GOPOOL: panic in pool: %s: %v: %s", p.name, r, debug.Stack())
			}
		}
	}(p, ctx)
	f()
}

func (p *GoPool) CurrentWorkers() int {
	return int(atomic.LoadInt32(&p.workers))
}

func (p *GoPool) runWorker() {
	id := atomic.AddInt32(&p.workers, 1)
	defer atomic.AddInt32(&p.workers, -1)

	if id > p.maxIdle {
		// drain task chan and exit without waiting
		for {
			select {
			case t := <-p.tasks:
				switch t.typ {
				case noopTask:
					// noop
				default:
					p.runTask(t.ctx, t.f)
				}
			default:
				return
			}
		}
	}

	createdAt := time.Now().UnixMilli() // for checking maxage
	for t := range p.tasks {
		switch t.typ {
		case noopTask:
			// noop
		default:
			p.runTask(t.ctx, t.f)
		}

		now := atomic.LoadInt64(&p.unixMilli)

		// check if ticker is NOT alive
		// p.unixMilli will be set to zero if it's not running
		if now == 0 {
			// cas and create a new ticker
			now = time.Now().UnixMilli()
			if atomic.CompareAndSwapInt64(&p.unixMilli, 0, now) {
				go p.runTicker()
			}
		}

		// check maxage
		if now-createdAt > p.maxage {
			return
		}
	}
}

func (p *GoPool) runTicker() {
	// mark it zero to trigger ticker to be created when we have active workers
	defer atomic.StoreInt64(&p.unixMilli, 0)

	// 1% precision as per maxage
	d := time.Duration(p.maxage) * time.Millisecond / 100

	// set a minimum value to avoid performance issues.
	if d < time.Millisecond {
		d = time.Millisecond
	}

	t := time.NewTicker(d)
	defer t.Stop()

	noop := task{f: func() {}, typ: noopTask}
	for now := range t.C {
		if p.CurrentWorkers() == 0 {
			return
		}
		atomic.StoreInt64(&p.unixMilli, now.UnixMilli())
		p.tasks <- noop
	}
}
