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

package unsafex

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUBinaryToString(t *testing.T) {
	b := []byte("hello")
	s := BinaryToString(b)
	assert.Equal(t, string(b), s)
	b[0] = 'x'
	assert.Equal(t, string(b), s)
}

func BenchmarkBinaryToString(b *testing.B) {
	x := []byte("hello")
	for i := 0; i < b.N; i++ {
		_ = BinaryToString(x)
	}
}

func TestStringToBinary(t *testing.T) {
	x := []byte("hello")
	// doesn't use string literal, or `b[0] = 'x'` will panic coz addr is readonly
	s := string(x)
	b := StringToBinary(s)
	assert.Equal(t, s, string(b))
	b[0] = 'x'
	assert.Equal(t, s, string(b))
}

func BenchmarkStringToBinary(b *testing.B) {
	s := "hello"
	for i := 0; i < b.N; i++ {
		_ = StringToBinary(s)
	}
}
