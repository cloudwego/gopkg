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

package gopool

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bytedance/gopkg/util/gopool"
)

func TestGoPool(t *testing.T) {
	{ // test normal case
		p := NewGoPool("TestGoPool", nil)
		n := 10
		wg := sync.WaitGroup{}
		wg.Add(n)
		v := int32(0)
		for i := 0; i < n; i++ {
			p.Go(func() {
				time.Sleep(time.Millisecond)
				atomic.AddInt32(&v, 1)
				wg.Done()
			})
		}
		wg.Wait()
		require.Equal(t, int32(n), atomic.LoadInt32(&v))
	}

	{ // test without PanicHandler
		p := NewGoPool("TestGoPool", nil)
		p.Go(func() { panic("x") })
		time.Sleep(time.Millisecond)
	}

	{ // test SetPanicHandler
		wg := sync.WaitGroup{}
		p := NewGoPool("TestGoPool", nil) // fix p.SetPanicHandler data race
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		x := "testpanic"
		p.SetPanicHandler(func(c context.Context, r interface{}) {
			defer wg.Done()
			require.Equal(t, x, r)
			require.Same(t, ctx, c)
		})
		wg.Add(1)
		p.CtxGo(ctx, func() {
			panic(x)
		})
		wg.Wait()
	}
}

func TestGoPool_Ticker(t *testing.T) {
	o := DefaultOption()
	o.WorkerMaxAge = 100 * time.Millisecond
	p := NewGoPool("TestGoPool_Ticker", o)
	for i := 0; i < 10; i++ {
		p.Go(func() { time.Sleep(o.WorkerMaxAge / 10) })
	}
	time.Sleep(10 * time.Millisecond) // wait all goroutines to run
	require.Equal(t, 10, p.CurrentWorkers())
	time.Sleep(o.WorkerMaxAge + o.WorkerMaxAge/10) // ticker will trigger worker to exit
	require.Equal(t, 0, p.CurrentWorkers())
}

func TestGoPool_Full(t *testing.T) {
	o := DefaultOption()
	o.TaskChanBuffer = 1 // smaller value, easier to be full.
	p := NewGoPool("TestGoPool_Full", o)

	v := int32(0)
	n := 10000
	for i := 0; i < n; i++ {
		p.Go(func() { atomic.AddInt32(&v, 1) })
	}
	time.Sleep(10 * time.Millisecond) // wait all goroutines done
	require.Equal(t, int32(n), atomic.LoadInt32(&v))
}

func TestGoPool_MaxIdle(t *testing.T) {
	o := DefaultOption()
	o.MaxIdleWorkers = 7
	p := NewGoPool("TestGoPool_MaxIdle", o)

	v := int32(0)
	n := 10000
	for i := 0; i < n; i++ {
		p.Go(func() { atomic.AddInt32(&v, 1) })
	}
	time.Sleep(10 * time.Millisecond) // wait all goroutines done
	require.Equal(t, int32(n), atomic.LoadInt32(&v))
	require.Equal(t, o.MaxIdleWorkers, p.CurrentWorkers())
}

// ======== Benchmarks ...

// must be const then make() will allocate on stack
const stacksize = 120

var (
	testDepths = []int{2, 32, 128}
	benchBatch = 2
)

func recursiveFunc(depth int) {
	if depth < 0 {
		return
	}
	b := make([]byte, stacksize)
	recursiveFunc(depth - 1)
	runtime.KeepAlive(b)
}

func makefunc(depth int, wg *sync.WaitGroup) func() {
	return func() {
		recursiveFunc(depth)
		wg.Done()
	}
}

func BenchmarkGoPool(b *testing.B) {
	newHandler := func(depth int, wg *sync.WaitGroup) func() {
		o := DefaultOption()
		p := NewGoPool("BenchmarkGoPool", o)
		f := makefunc(depth, wg)
		return func() {
			p.Go(f)
		}
	}
	benchmarkGo(newHandler, b)
}

func BenchmarkBytedanceGoPool(b *testing.B) {
	newHandler := func(depth int, wg *sync.WaitGroup) func() {
		p := gopool.NewPool("BenchmarkBytedanceGoPool", math.MaxInt32, gopool.NewConfig())
		f := makefunc(depth, wg)
		return func() {
			p.Go(f)
		}
	}
	benchmarkGo(newHandler, b)
}

func BenchmarkGoWithoutPool(b *testing.B) {
	newHandler := func(depth int, wg *sync.WaitGroup) func() {
		p := &GoPool{}
		f := makefunc(depth, wg)
		testf := func() {
			// reuse runTask method
			p.runTask(context.Background(), f)
		}
		return func() {
			go testf()
		}
	}
	benchmarkGo(newHandler, b)
}

func benchmarkGo(newHandler func(int, *sync.WaitGroup) func(), b *testing.B) {
	for _, depth := range testDepths {
		b.Run(fmt.Sprintf("batch_%d_stacksize_%d", benchBatch, depth*stacksize), func(b *testing.B) {
			b.RunParallel(func(pb *testing.PB) {
				var wg sync.WaitGroup
				f := newHandler(depth, &wg)
				for pb.Next() {
					wg.Add(benchBatch)
					for i := 0; i < benchBatch; i++ {
						f()
					}
					wg.Wait()
				}
			})
		})
	}
}
