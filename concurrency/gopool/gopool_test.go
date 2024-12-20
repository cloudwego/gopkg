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

	// test SetPanicHandler
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	x := "testpanic"
	p.SetPanicHandler(func(c context.Context, r interface{}) {
		defer wg.Done()
		require.Equal(t, x, r)
		require.Same(t, ctx, c)
	})
	wg.Add(1)
	p.GoCtx(ctx, func() {
		panic(x)
	})
	wg.Wait()

}

func TestGoPool_Ticker(t *testing.T) {
	o := DefaultOption()
	o.WorkerMaxAge = 50 * time.Millisecond
	p := NewGoPool("TestGoPool_Ticker", o)
	for i := 0; i < 10; i++ {
		p.Go(func() { time.Sleep(o.WorkerMaxAge) })
	}
	time.Sleep(o.WorkerMaxAge / 10) // wait all goroutines to run
	require.Equal(t, 10, p.CurrentWorkers())
	time.Sleep(2 * o.WorkerMaxAge) // ticker will trigger worker to exit
	require.Equal(t, 0, p.CurrentWorkers())
}

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

// must be const then make() will allocate on stack
const stacksize = 120

var (
	testDepths = []int{2, 32, 128}
	benchBatch = 5
)

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
