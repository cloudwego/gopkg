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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBinaryReader(t *testing.T) {
	x := BinaryProtocol{}
	b := x.AppendMessageBegin(nil, "hello", 1, 2)
	sz0 := len(b)
	b = x.AppendFieldBegin(b, 3, 4)
	sz1 := len(b)
	b = x.AppendFieldStop(b)
	sz2 := len(b)
	b = x.AppendMapBegin(b, 5, 6, 7)
	sz3 := len(b)
	b = x.AppendListBegin(b, 8, 9)
	sz4 := len(b)
	b = x.AppendSetBegin(b, 10, 11)
	sz5 := len(b)
	b = x.AppendBinary(b, []byte("12"))
	sz6 := len(b)
	b = x.AppendString(b, "13")
	sz7 := len(b)
	b = x.AppendBool(b, true)
	b = x.AppendBool(b, false)
	sz8 := len(b)
	b = x.AppendByte(b, 14)
	sz9 := len(b)
	b = x.AppendI16(b, 15)
	sz10 := len(b)
	b = x.AppendI32(b, 16)
	sz11 := len(b)
	b = x.AppendI64(b, 17)
	sz12 := len(b)
	b = x.AppendDouble(b, 18.5)
	sz13 := len(b)

	r := NewBinaryReader(bytes.NewReader(b))
	defer r.Release()
	name, mt, seq, err := r.ReadMessageBegin()
	require.NoError(t, err)
	require.Equal(t, "hello", name)
	require.Equal(t, TMessageType(1), mt)
	require.Equal(t, int32(2), seq)
	require.Equal(t, sz0, int(r.Readn()))

	ft, fid, err := r.ReadFieldBegin()
	require.NoError(t, err)
	require.Equal(t, TType(3), ft)
	require.Equal(t, int16(4), fid)
	require.Equal(t, sz1, int(r.Readn()))

	ft, fid, err = r.ReadFieldBegin() // for AppendFieldStop
	require.NoError(t, err)
	require.Equal(t, STOP, ft)
	require.Equal(t, int16(0), fid)
	require.Equal(t, sz2, int(r.Readn()))

	kt, vt, sz, err := r.ReadMapBegin()
	require.NoError(t, err)
	require.Equal(t, TType(5), kt)
	require.Equal(t, TType(6), vt)
	require.Equal(t, int(7), sz)
	require.Equal(t, sz3, int(r.Readn()))

	et, sz, err := r.ReadListBegin()
	require.NoError(t, err)
	require.Equal(t, TType(8), et)
	require.Equal(t, int(9), sz)
	require.Equal(t, sz4, int(r.Readn()))

	et, sz, err = r.ReadSetBegin()
	require.NoError(t, err)
	require.Equal(t, TType(10), et)
	require.Equal(t, int(11), sz)
	require.Equal(t, sz5, int(r.Readn()))

	bin, err := r.ReadBinary()
	require.NoError(t, err)
	require.Equal(t, "12", string(bin))
	require.Equal(t, sz6, int(r.Readn()))

	s, err := r.ReadString()
	require.NoError(t, err)
	require.Equal(t, "13", s)
	require.Equal(t, sz7, int(r.Readn()))

	vb, err := r.ReadBool()
	require.NoError(t, err)
	require.True(t, vb)
	vb, err = r.ReadBool()
	require.NoError(t, err)
	require.False(t, vb)
	require.Equal(t, sz8, int(r.Readn()))

	v8, err := r.ReadByte()
	require.NoError(t, err)
	require.Equal(t, int8(14), v8)
	require.Equal(t, sz9, int(r.Readn()))

	v16, err := r.ReadI16()
	require.NoError(t, err)
	require.Equal(t, int16(15), v16)
	require.Equal(t, sz10, int(r.Readn()))

	v32, err := r.ReadI32()
	require.NoError(t, err)
	require.Equal(t, int32(16), v32)
	require.Equal(t, sz11, int(r.Readn()))

	v64, err := r.ReadI64()
	require.NoError(t, err)
	require.Equal(t, int64(17), v64)
	require.Equal(t, sz12, int(r.Readn()))

	vf, err := r.ReadDouble()
	require.NoError(t, err)
	require.Equal(t, float64(18.5), vf)
	require.Equal(t, sz13, int(r.Readn()))
}

func TestBinaryReaderSkip(t *testing.T) {
	x := BinaryProtocol{}
	// byte
	b := x.AppendByte([]byte(nil), 1)
	sz0 := len(b)

	// string
	b = x.AppendString(b, "hello")
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

	r := NewBinaryReader(bytes.NewReader(b))
	defer r.Release()

	err := r.Skip(BYTE) // byte
	require.NoError(t, err)
	require.Equal(t, int64(sz0), r.Readn())
	err = r.Skip(STRING) // string
	require.NoError(t, err)
	require.Equal(t, int64(sz1), r.Readn())
	err = r.Skip(LIST) // list<i32>
	require.NoError(t, err)
	require.Equal(t, int64(sz2), r.Readn())
	err = r.Skip(LIST) // list<string>
	require.NoError(t, err)
	require.Equal(t, int64(sz3), r.Readn())
	err = r.Skip(LIST) // list<list<i32>>
	require.NoError(t, err)
	require.Equal(t, int64(sz4), r.Readn())
	err = r.Skip(MAP) // map<i32, i64>
	require.NoError(t, err)
	require.Equal(t, int64(sz5), r.Readn())
	err = r.Skip(MAP) // map<i32, string>
	require.NoError(t, err)
	require.Equal(t, int64(sz6), r.Readn())
	err = r.Skip(MAP) // map<string, i64>
	require.NoError(t, err)
	require.Equal(t, int64(sz7), r.Readn())
	err = r.Skip(MAP) // map<i32, list<i32>>
	require.NoError(t, err)
	require.Equal(t, int64(sz8), r.Readn())
	err = r.Skip(MAP) // map<list<i32>, i32>
	require.NoError(t, err)
	require.Equal(t, int64(sz9), r.Readn())
	err = r.Skip(STRUCT) // struct i32, list<i32>
	require.NoError(t, err)
	require.Equal(t, int64(sz10), r.Readn())

	{ // other cases
		// errDepthLimitExceeded
		b = b[:0]
		for i := 0; i < defaultRecursionDepth+1; i++ {
			b = x.AppendFieldBegin(b, STRUCT, 1)
		}
		r := NewBinaryReader(bytes.NewReader(b))
		err := r.Skip(STRUCT)
		require.Same(t, errDepthLimitExceeded, err)

		// unknown type
		err = r.Skip(TType(122))
		require.Error(t, err)
	}
}
