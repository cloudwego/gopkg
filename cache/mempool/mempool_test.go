/*
 * Copyright 2024 CloudWeGo Authors
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

package mempool

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
)

func TestMallocFree(t *testing.T) {
	for i := 127; i < 1<<20; i += 1000 { //  it tests malloc 127B - 1MB, with step 1000
		b := Malloc(i)
		require.Equal(t, i, len(b))
		Free(b)
	}
}

func TestCap(t *testing.T) {
	sz8k := 8 << 10
	b := Malloc(sz8k)
	require.Greater(t, Cap(b), sz8k)
	Free(b)

	b = Malloc(sz8k - footerLen)
	require.Equal(t, sz8k-footerLen, Cap(b))
	require.Equal(t, sz8k, cap(b))
	Free(b)
}

func TestAppend(t *testing.T) {
	str := "TestAppend"
	b := Malloc(0)
	for i := 0; i < 2000; i++ {
		b = Append(b, []byte(str)...)
	}
	Free(b)

	str = "TestAppendStr"
	b = Malloc(0)
	for i := 0; i < 2000; i++ {
		b = AppendStr(b, str)
	}
	Free(b)
}

func TestFree(t *testing.T) {
	minsz := minMemPoolSize

	Free([]byte{})                     // case: cap == 0
	Free(make([]byte, 0, minsz+1))     // case: not power of two
	Free(make([]byte, minsz-1, minsz)) // case: < footerLen

	b := make([]byte, minsz-footerLen, minsz)
	footer := make([]byte, footerLen)

	Free(b) // case: magic err

	*(*uint64)(unsafe.Pointer(&footer[0])) = footerMagic | 1
	_ = append(b, footer...)
	Free(b) // case: index err

	*(*uint64)(unsafe.Pointer(&footer[0])) = footerMagic | 0
	_ = append(b, footer...)
	Free(b) // all good
}

func TestResetFooter(t *testing.T) {
	b := Malloc(1)
	x := getFooter(b)
	require.True(t, x != 0)
	resetFooter(b)
	y := getFooter(b)
	require.True(t, y == 0)
}

func Benchmark_MallocFree(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			b := Malloc(i & 0xffff)
			Free(b)
			i++
		}
	})
}

func Benchmark_AppendStr(b *testing.B) {
	str := "Benchmark_AppendStr"
	b.ReportAllocs()
	b.SetBytes(int64(len(str)))
	b.RunParallel(func(pb *testing.PB) {
		i := 1
		b := Malloc(1)
		for pb.Next() {
			if i&0xff == 0 { // 255 * len(str) ~ 4845 > minMemPoolSize
				Free(b)
				b = Malloc(1)
			}
			b = AppendStr(b, str)
			i++
		}
		Free(b)
	})
}
