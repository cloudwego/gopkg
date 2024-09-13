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

package xfnv

import (
	"crypto/rand"
	"fmt"
	"hash/maphash"
	"testing"

	"github.com/bytedance/gopkg/util/xxhash3"
	"github.com/stretchr/testify/require"
)

func TestHashStr(t *testing.T) {
	require.Equal(t, HashStr("1234"), HashStr("1234"))
	require.NotEqual(t, HashStr("12345"), HashStr("12346"))
	require.Equal(t, HashStr("12345678"), HashStr("12345678"))
	require.NotEqual(t, HashStr("123456789"), HashStr("123456788"))
}

func BenchmarkHash(b *testing.B) {
	sizes := []int{8, 16, 32, 64, 128, 512}
	bb := make([][]byte, len(sizes))
	for i := range bb {
		b := make([]byte, sizes[i])
		rand.Read(b)
		bb[i] = b
	}
	b.ResetTimer()
	for _, data := range bb {
		b.Run(fmt.Sprintf("size-%d-xfnv", len(data)), func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			for i := 0; i < b.N; i++ {
				_ = Hash(data)
			}
		})
	}

	println("")

	for _, data := range bb {
		b.Run(fmt.Sprintf("size-%d-xxhash3", len(data)), func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			for i := 0; i < b.N; i++ {
				_ = xxhash3.Hash(data)
			}
		})
	}

	println("")

	for _, data := range bb {
		b.Run(fmt.Sprintf("size-%d-maphash", len(data)), func(b *testing.B) {
			s := maphash.MakeSeed()
			h := &maphash.Hash{}
			h.SetSeed(s)
			b.SetBytes(int64(len(data)))
			for i := 0; i < b.N; i++ {
				// use maphash.Bytes which is more fair to benchmark after go1.19
				// maphash.Bytes(s, data)
				_, _ = h.Write(data)
			}
		})
	}
}
