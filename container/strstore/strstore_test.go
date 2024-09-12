// Copyright 2024 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package strstore

import (
	"math/rand"
	_ "net/http/pprof"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStrStore(t *testing.T) {
	// test when the pages grow
	ss := randStrings(50, 1000000)
	strStore, idxes := New(ss)
	for i := 0; i < len(ss); i++ {
		assert.Equal(t, ss[i], strStore.Get(idxes[i]))
	}
	s := strStore.Get(-1)
	assert.Equal(t, "", s)
	s = strStore.Get(strStore.Len() * 2)
	assert.Equal(t, "", s)
}

func BenchmarkStrStoreGetSet(b *testing.B) {
	ss := randStrings(50, 1000000)
	strStore, idxes := New(ss)
	strSlice := make([]string, 0, len(ss))
	for i := 0; i < len(ss); i++ {
		strSlice = append(strSlice, ss[i])
	}

	b.Run("strbuf-get", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			strStore.Get(idxes[0])
		}
	})

	b.Run("stdstrslice-get", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = strSlice[0]
		}
	})
}

func BenchmarkStrStoreGC(b *testing.B) {
	ss := randStrings(50, 1000000)
	strStore, idxes := New(ss)
	_ = ss
	runtime.GC()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		runtime.GC()
	}
	runtime.KeepAlive(strStore)
	runtime.KeepAlive(idxes)
}

func BenchmarkStdStrSliceGC(b *testing.B) {
	ss := randStrings(50, 1000000)
	strSlice := make([]string, 0, len(ss))
	for i := 0; i < len(ss); i++ {
		strSlice = append(strSlice, ss[i])
	}
	_ = ss
	runtime.GC()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		runtime.GC()
	}
	runtime.KeepAlive(strSlice)
}

func randStrings(m, n int) []string {
	b := make([]byte, m*n)
	rand.Read(b)
	ret := make([]string, 0, n)
	for i := 0; i < n; i++ {
		s := b[m*i:]
		s = s[:m]
		ret = append(ret, string(s))
	}
	return ret
}
