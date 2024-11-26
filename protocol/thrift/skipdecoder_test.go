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
	"strings"
	"testing"

	"github.com/cloudwego/gopkg/bufiox"
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

	r := NewSkipDecoder(bufiox.NewBytesReader(b))
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
		r := NewSkipDecoder(bufiox.NewBytesReader(b))
		_, err := r.Next(STRUCT)
		require.Same(t, errDepthLimitExceeded, err)

		// unknown type
		_, err = r.Next(TType(122))
		require.Error(t, err)
	}
}

var mockString = make([]byte, 5000)

func BenchmarkSkipDecoder(b *testing.B) {
	// prepare data
	bs := make([]byte, 0, 1024)

	// BOOL, fid=1
	bs = Binary.AppendFieldBegin(bs, BOOL, 1)
	bs = Binary.AppendBool(bs, true)

	// BYTE, fid=2
	bs = Binary.AppendFieldBegin(bs, BYTE, 2)
	bs = Binary.AppendByte(bs, 2)

	// I16, fid=3
	bs = Binary.AppendFieldBegin(bs, I16, 3)
	bs = Binary.AppendI16(bs, 3)

	// I32, fid=4
	bs = Binary.AppendFieldBegin(bs, I32, 4)
	bs = Binary.AppendI32(bs, 4)

	// I64, fid=5
	bs = Binary.AppendFieldBegin(bs, I64, 5)
	bs = Binary.AppendI64(bs, 5)

	// DOUBLE, fid=6
	bs = Binary.AppendFieldBegin(bs, DOUBLE, 6)
	bs = Binary.AppendDouble(bs, 6)

	// STRING, fid=7
	bs = Binary.AppendFieldBegin(bs, STRING, 7)
	bs = Binary.AppendString(bs, string(mockString))

	// MAP, fid=8
	bs = Binary.AppendFieldBegin(bs, MAP, 8)
	bs = Binary.AppendMapBegin(bs, DOUBLE, DOUBLE, 1)
	bs = Binary.AppendDouble(bs, 8.1)
	bs = Binary.AppendDouble(bs, 8.2)

	// SET, fid=9
	bs = Binary.AppendFieldBegin(bs, SET, 9)
	bs = Binary.AppendSetBegin(bs, I64, 1)
	bs = Binary.AppendI64(bs, 9)

	// LIST, fid=10
	bs = Binary.AppendFieldBegin(bs, LIST, 10)
	bs = Binary.AppendListBegin(bs, I64, 1)
	bs = Binary.AppendI64(bs, 10)

	// STRUCT with 1 field I64, fid=11,1
	bs = Binary.AppendFieldBegin(bs, STRUCT, 11)
	bs = Binary.AppendFieldBegin(bs, I64, 1)
	bs = Binary.AppendI64(bs, 11)
	bs = Binary.AppendFieldStop(bs)

	// Finish struct
	bs = Binary.AppendFieldStop(bs)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			bufReader := bufiox.NewBytesReader(bs)
			sr := NewSkipDecoder(bufReader)
			buf, err := sr.Next(STRUCT)
			if err != nil {
				b.Fatal(err)
			}
			if !bytes.Equal(buf, bs) {
				b.Fatal("bytes not equal")
			}
			sr.Release()
			_ = bufReader.Release(nil)
		}
	})
}
