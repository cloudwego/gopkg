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

package strmap

import (
	"crypto/rand"
	"fmt"
	"runtime"
	"testing"

	"github.com/cloudwego/gopkg/internal/hack"
	"github.com/stretchr/testify/require"
)

func randStrings(m, n int) []string {
	b := make([]byte, m*n)
	rand.Read(b)
	ret := make([]string, 0, n)
	for i := 0; i < n; i++ {
		s := b[m*i:]
		s = s[:m]
		ret = append(ret, hack.ByteSliceToString(s))
	}
	return ret
}

// newStdStrMap generates a map with uniq values
func newStdStrMap(ss []string) map[string]uint {
	v := uint(1)
	m := make(map[string]uint)
	for _, s := range ss {
		_, ok := m[s]
		if !ok {
			m[s] = v
			v++
		}
	}
	return m
}

func TestStrMap(t *testing.T) {
	ss := randStrings(20, 100000)
	m := newStdStrMap(ss)
	sm := New(m)
	require.Equal(t, len(m), sm.Len())
	for i, s := range ss {
		v0 := m[s]
		v1, _ := sm.Get(s)
		require.Equal(t, v0, v1, i)
	}
	for i, s := range randStrings(20, 100000) {
		v0, ok0 := m[s]
		v1, ok1 := sm.Get(s)
		require.Equal(t, ok0, ok1, i)
		require.Equal(t, v0, v1, i)
	}
	m0 := make(map[string]uint)
	for i := 0; i < sm.Len(); i++ {
		s, v := sm.Item(i)
		m0[s] = v
	}
	require.Equal(t, m, m0)
}

func TestStrMapString(t *testing.T) {
	ss := []string{"a", "b", "c"}
	m := newStdStrMap(ss)
	sm := New(m)
	t.Log(sm.String())
	t.Log(sm.debugString())
}

func BenchmarkGet(b *testing.B) {
	sizes := []int{20, 50, 100}
	nn := []int{100000, 200000}

	for _, n := range nn {
		for _, sz := range sizes {
			ss := randStrings(sz, n)
			m := newStdStrMap(ss)
			b.Run(fmt.Sprintf("std-keysize_%d_n_%d", sz, n), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					_ = m[ss[i%len(ss)]]
				}
			})
			b.Run(fmt.Sprintf("new-keysize_%d_n_%d", sz, n), func(b *testing.B) {
				sm := New(m)
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					sm.Get(ss[i%len(ss)])
				}
			})
		}
	}
}

func BenchmarkGC(b *testing.B) {
	sizes := []int{20, 100}
	nn := []int{100000, 400000}

	for _, n := range nn {
		for _, sz := range sizes {
			ss := randStrings(sz, n)
			m := newStdStrMap(ss)
			b.Run(fmt.Sprintf("std-keysize_%d_n_%d", sz, n), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					runtime.GC()
				}
			})

			sm := New(m)
			m = nil
			runtime.GC()

			b.Run(fmt.Sprintf("new-keysize_%d_n_%d", sz, n), func(b *testing.B) {
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					runtime.GC()
				}
			})

			_ = m // fix lint ineffassign of m = nil
			runtime.KeepAlive(sm)
		}
	}
}
