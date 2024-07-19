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

package thrift

import (
	"bytes"
	"math/rand"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSkipDecoder(t *testing.T) {
	x := BinaryProtocol{}
	// byte
	b := x.AppendByte([]byte(nil), 1)
	sz0 := len(b)

	// string
	b = x.AppendString(b, strings.Repeat("hello", 500)) // larger than buffer
	sz1 := len(b)

	// list<i32>
	b = x.AppendListBegin(b, I32, 1)
	b = x.AppendI32(b, 1)
	sz2 := len(b)

	// list<string>
	b = x.AppendListBegin(b, STRING, 1)
	b = x.AppendString(b, "hello")
	sz3 := len(b)

	// list<list<i32>>
	b = x.AppendListBegin(b, LIST, 1)
	b = x.AppendListBegin(b, I32, 1)
	b = x.AppendI32(b, 1)
	sz4 := len(b)

	// map<i32, i64>
	b = x.AppendMapBegin(b, I32, I64, 1)
	b = x.AppendI32(b, 1)
	b = x.AppendI64(b, 2)
	sz5 := len(b)

	// map<i32, string>
	b = x.AppendMapBegin(b, I32, STRING, 1)
	b = x.AppendI32(b, 1)
	b = x.AppendString(b, "hello")
	sz6 := len(b)

	// map<string, i64>
	b = x.AppendMapBegin(b, STRING, I64, 1)
	b = x.AppendString(b, "hello")
	b = x.AppendI64(b, 2)
	sz7 := len(b)

	// map<i32, list<i32>>
	b = x.AppendMapBegin(b, I32, LIST, 1)
	b = x.AppendI32(b, 1)
	b = x.AppendListBegin(b, I32, 1)
	b = x.AppendI32(b, 1)
	sz8 := len(b)

	// map<list<i32>, i32>
	b = x.AppendMapBegin(b, LIST, I32, 1)
	b = x.AppendListBegin(b, I32, 1)
	b = x.AppendI32(b, 1)
	b = x.AppendI32(b, 1)
	sz9 := len(b)

	// struct i32, list<i32>
	b = x.AppendFieldBegin(b, I32, 1)
	b = x.AppendI32(b, 1)
	b = x.AppendFieldBegin(b, LIST, 1)
	b = x.AppendListBegin(b, I32, 1)
	b = x.AppendI32(b, 1)
	b = x.AppendFieldStop(b)
	sz10 := len(b)

	r := NewSkipDecoder(bytes.NewReader(b))
	defer r.Release()

	readn := 0
	b, err := r.Next(BYTE) // byte
	require.NoError(t, err)
	readn += len(b)
	require.Equal(t, sz0, readn)
	b, err = r.Next(STRING) // string
	require.NoError(t, err)
	readn += len(b)
	require.Equal(t, sz1, readn)
	b, err = r.Next(LIST) // list<i32>
	require.NoError(t, err)
	readn += len(b)
	require.Equal(t, sz2, readn)
	b, err = r.Next(LIST) // list<string>
	require.NoError(t, err)
	readn += len(b)
	require.Equal(t, sz3, readn)
	b, err = r.Next(LIST) // list<list<i32>>
	require.NoError(t, err)
	readn += len(b)
	require.Equal(t, sz4, readn)
	b, err = r.Next(MAP) // map<i32, i64>
	require.NoError(t, err)
	readn += len(b)
	require.Equal(t, sz5, readn)
	b, err = r.Next(MAP) // map<i32, string>
	require.NoError(t, err)
	readn += len(b)
	require.Equal(t, sz6, readn)
	b, err = r.Next(MAP) // map<string, i64>
	require.NoError(t, err)
	readn += len(b)
	require.Equal(t, sz7, readn)
	b, err = r.Next(MAP) // map<i32, list<i32>>
	require.NoError(t, err)
	readn += len(b)
	require.Equal(t, sz8, readn)
	b, err = r.Next(MAP) // map<list<i32>, i32>
	require.NoError(t, err)
	readn += len(b)
	require.Equal(t, sz9, readn)
	b, err = r.Next(STRUCT) // struct i32, list<i32>
	require.NoError(t, err)
	readn += len(b)
	require.Equal(t, sz10, readn)

	{ // other cases
		// errDepthLimitExceeded
		b = b[:0]
		for i := 0; i < defaultRecursionDepth+1; i++ {
			b = x.AppendFieldBegin(b, STRUCT, 1)
		}
		r := NewSkipDecoder(bytes.NewReader(b))
		_, err := r.Next(STRUCT)
		require.Same(t, errDepthLimitExceeded, err)

		// unknown type
		_, err = r.Next(TType(122))
		require.Error(t, err)
	}
}

func TestSkipDecoderReset(t *testing.T) {
	x := BinaryProtocol{}
	b := x.AppendString([]byte(nil), "hello")

	r := NewSkipDecoder(nil)
	for i := 0; i < 10; i++ {
		if rand.Intn(2) == 1 { // random skipreader to test Reset
			r.Reset(&remoteByteBufferImplForT{b: b})
		} else {
			r.Reset(bytes.NewReader(b))
		}
		retb, err := r.Next(STRING)
		require.NoError(t, err)
		require.Equal(t, b, retb)
	}
}
