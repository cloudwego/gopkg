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
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudwego/gopkg/internal/testutils/netpoll"
)

func TestBinary(t *testing.T) {
	{ // Bool
		sz := 2 * Binary.BoolLength()

		b := Binary.AppendBool([]byte(nil), true)
		b = Binary.AppendBool(b, false)
		require.Equal(t, sz, len(b))

		b1 := make([]byte, sz)
		l := Binary.WriteBool(b1, true)
		l += Binary.WriteBool(b1[l:], false)
		require.Equal(t, sz, l)
		require.Equal(t, b, b1)

		v, l, _ := Binary.ReadBool(b)
		require.Equal(t, 1, l)
		require.True(t, v)
		v, l, _ = Binary.ReadBool(b[1:])
		require.Equal(t, 1, l)
		require.False(t, v)

		_, _, err := Binary.ReadBool([]byte(nil))
		require.Same(t, errReadBool, err)
	}

	{ // Byte
		sz := Binary.ByteLength()

		b := Binary.AppendByte([]byte(nil), 1)
		require.Equal(t, sz, len(b))

		b1 := make([]byte, sz)
		l := Binary.WriteByte(b1, 1)
		require.Equal(t, sz, l)
		require.Equal(t, b, b1)

		v, l, _ := Binary.ReadByte(b)
		require.Equal(t, 1, l)
		require.Equal(t, int8(1), v)

		_, _, err := Binary.ReadByte([]byte(nil))
		require.Same(t, errReadByte, err)
	}

	{ // I16
		testv := int16(0x7f)
		sz := Binary.I16Length()

		b := Binary.AppendI16([]byte(nil), testv)
		require.Equal(t, sz, len(b))

		b1 := make([]byte, sz)
		l := Binary.WriteI16(b1, testv)
		require.Equal(t, sz, l)
		require.Equal(t, b, b1)

		v, l, _ := Binary.ReadI16(b)
		require.Equal(t, sz, l)
		require.Equal(t, testv, v)

		_, _, err := Binary.ReadI16([]byte(nil))
		require.Same(t, errReadI16, err)
	}

	{ // I32
		testv := int32(0x7fffffff)
		sz := Binary.I32Length()

		b := Binary.AppendI32([]byte(nil), testv)
		require.Equal(t, sz, len(b))

		b1 := make([]byte, sz)
		l := Binary.WriteI32(b1, testv)
		require.Equal(t, sz, l)
		require.Equal(t, b, b1)

		v, l, _ := Binary.ReadI32(b)
		require.Equal(t, sz, l)
		require.Equal(t, testv, v)

		_, _, err := Binary.ReadI32([]byte(nil))
		require.Same(t, errReadI32, err)
	}

	{ // I64
		testv := int64(0x7fffffff7fffffff)
		sz := Binary.I64Length()

		b := Binary.AppendI64([]byte(nil), testv)
		require.Equal(t, sz, len(b))

		b1 := make([]byte, sz)
		l := Binary.WriteI64(b1, testv)
		require.Equal(t, sz, l)
		require.Equal(t, b, b1)

		v, l, _ := Binary.ReadI64(b)
		require.Equal(t, sz, l)
		require.Equal(t, testv, v)

		_, _, err := Binary.ReadI64([]byte(nil))
		require.Same(t, errReadI64, err)
	}

	{ // Double
		testv := float64(0.125)
		sz := Binary.DoubleLength()

		b := Binary.AppendDouble([]byte(nil), testv)
		require.Equal(t, sz, len(b))

		b1 := make([]byte, sz)
		l := Binary.WriteDouble(b1, testv)
		require.Equal(t, sz, l)
		require.Equal(t, b, b1)

		v, l, _ := Binary.ReadDouble(b)
		require.Equal(t, sz, l)
		require.Equal(t, testv, v)

		_, _, err := Binary.ReadDouble([]byte(nil))
		require.Same(t, errReadDouble, err)
	}

	{ // Binary
		testv := []byte("hello")
		sz := Binary.BinaryLength(testv)

		b := Binary.AppendBinary([]byte(nil), testv)
		require.Equal(t, sz, len(b))

		b1 := make([]byte, sz)
		l := Binary.WriteBinaryNocopy(b1, nil, testv)
		require.Equal(t, sz, l)
		require.Equal(t, b, b1)

		v, l, _ := Binary.ReadBinary(b)
		require.Equal(t, sz, l)
		require.Equal(t, testv, v)

		_, _, err := Binary.ReadBinary([]byte(nil))
		require.Same(t, errReadBin, err)
	}

	{ // String
		testv := "hello"
		sz := Binary.StringLength(testv)

		b := Binary.AppendString([]byte(nil), testv)
		require.Equal(t, sz, len(b))

		b1 := make([]byte, sz)
		l := Binary.WriteStringNocopy(b1, nil, testv)
		require.Equal(t, sz, l)
		require.Equal(t, b, b1)

		v, l, _ := Binary.ReadString(b)
		require.Equal(t, sz, l)
		require.Equal(t, testv, v)

		_, _, err := Binary.ReadString([]byte(nil))
		require.Same(t, errReadStr, err)
	}

	{ // Message
		testname, testtype, testseq := "name", CALL, int32(7)
		sz := Binary.MessageBeginLength(testname)

		b := Binary.AppendMessageBegin([]byte(nil), testname, testtype, testseq)
		require.Equal(t, sz, len(b))

		b1 := make([]byte, sz)
		l := Binary.WriteMessageBegin(b1, testname, testtype, testseq)
		require.Equal(t, sz, l)
		require.Equal(t, b, b1)

		name, typeid, seq, l, _ := Binary.ReadMessageBegin(b)
		require.Equal(t, sz, l)
		require.Equal(t, testname, name)

		require.Equal(t, testtype, typeid)
		require.Equal(t, testseq, seq)

		_, _, _, _, err := Binary.ReadMessageBegin([]byte(nil))
		require.Same(t, errReadMessage, err)
	}

	{ // Field
		testtype, testfid := I64, int16(7)
		sz := Binary.FieldBeginLength() + Binary.FieldStopLength()

		b := Binary.AppendFieldBegin([]byte(nil), testtype, testfid)
		b = Binary.AppendFieldStop(b)
		require.Equal(t, sz, len(b))

		b1 := make([]byte, sz)
		l := Binary.WriteFieldBegin(b1, testtype, testfid)
		l += Binary.WriteFieldStop(b1[l:])
		require.Equal(t, sz, l)
		require.Equal(t, b, b1)

		typeid, fid, l, _ := Binary.ReadFieldBegin(b)
		require.Equal(t, sz, l+1) // +STOP
		require.Equal(t, testtype, typeid)
		require.Equal(t, testfid, fid)

		typeid, _, l, err := Binary.ReadFieldBegin(b[l:])
		require.NoError(t, err)
		require.Equal(t, 1, l)
		require.Equal(t, STOP, typeid)

		_, _, _, err = Binary.ReadFieldBegin([]byte(nil))
		require.Same(t, errReadField, err)
	}

	{ // Map
		testkt, testvt, testsize := I64, I32, 7
		sz := Binary.MapBeginLength()

		b := Binary.AppendMapBegin([]byte(nil), testkt, testvt, testsize)
		require.Equal(t, sz, len(b))

		b1 := make([]byte, sz)
		l := Binary.WriteMapBegin(b1, testkt, testvt, testsize)
		require.Equal(t, sz, l)
		require.Equal(t, b, b1)

		kt, vt, size, l, _ := Binary.ReadMapBegin(b)
		require.Equal(t, sz, l)
		require.Equal(t, testkt, kt)
		require.Equal(t, testvt, vt)
		require.Equal(t, testsize, size)

		_, _, _, _, err := Binary.ReadMapBegin([]byte(nil))
		require.Same(t, errReadMap, err)
	}

	{ // List
		testvt, testsize := I32, 7
		sz := Binary.ListBeginLength()

		b := Binary.AppendListBegin([]byte(nil), testvt, testsize)
		require.Equal(t, sz, len(b))

		b1 := make([]byte, sz)
		l := Binary.WriteListBegin(b1, testvt, testsize)
		require.Equal(t, sz, l)
		require.Equal(t, b, b1)

		vt, size, l, _ := Binary.ReadListBegin(b)
		require.Equal(t, sz, l)
		require.Equal(t, testvt, vt)
		require.Equal(t, testsize, size)

		_, _, _, err := Binary.ReadListBegin([]byte(nil))
		require.Same(t, errReadList, err)
	}

	{ // Set
		testvt, testsize := I32, 7
		sz := Binary.SetBeginLength()

		b := Binary.AppendSetBegin([]byte(nil), testvt, testsize)
		require.Equal(t, sz, len(b))

		b1 := make([]byte, sz)
		l := Binary.WriteSetBegin(b1, testvt, testsize)
		require.Equal(t, sz, l)
		require.Equal(t, b, b1)

		vt, size, l, _ := Binary.ReadSetBegin(b)
		require.Equal(t, sz, l)
		require.Equal(t, testvt, vt)
		require.Equal(t, testsize, size)

		_, _, _, err := Binary.ReadSetBegin([]byte(nil))
		require.Same(t, errReadSet, err)
	}
}

func TestBinary_ErrDataLength(t *testing.T) {
	x := BinaryProtocol{}
	{ // String
		b := x.AppendI32([]byte(nil), -1)
		_, _, err := x.ReadString(b)
		require.Same(t, errDataLength, err)
	}

	{ // Binary
		b := x.AppendI32([]byte(nil), -1)
		_, _, err := x.ReadBinary(b)
		require.Same(t, errDataLength, err)
	}

	{ // Map
		testkt, testvt, testsize := I64, I32, -1
		b := x.AppendMapBegin([]byte(nil), testkt, testvt, testsize)
		_, _, _, _, err := x.ReadMapBegin(b)
		require.Same(t, errDataLength, err)
	}

	{ // List
		testvt, testsize := I32, -1
		b := x.AppendListBegin([]byte(nil), testvt, testsize)
		_, _, _, err := x.ReadListBegin(b)
		require.Same(t, errDataLength, err)
	}

	{ // Set
		testvt, testsize := I32, -1
		b := x.AppendSetBegin([]byte(nil), testvt, testsize)
		_, _, _, err := x.ReadSetBegin(b)
		require.Same(t, errDataLength, err)
	}
}

func TestBinarySkip(t *testing.T) {
	// byte
	b := Binary.AppendByte([]byte(nil), 1)

	// string
	b = Binary.AppendString(b, "hello")

	// list<i32>
	b = Binary.AppendListBegin(b, I32, 1)
	b = Binary.AppendI32(b, 1)

	// list<string>
	b = Binary.AppendListBegin(b, STRING, 1)
	b = Binary.AppendString(b, "hello")

	// list<list<i32>>
	b = Binary.AppendListBegin(b, LIST, 1)
	b = Binary.AppendListBegin(b, I32, 1)
	b = Binary.AppendI32(b, 1)

	// map<i32, i64>
	b = Binary.AppendMapBegin(b, I32, I64, 1)
	b = Binary.AppendI32(b, 1)
	b = Binary.AppendI64(b, 2)

	// map<i32, string>
	b = Binary.AppendMapBegin(b, I32, STRING, 1)
	b = Binary.AppendI32(b, 1)
	b = Binary.AppendString(b, "hello")

	// map<string, i64>
	b = Binary.AppendMapBegin(b, STRING, I64, 1)
	b = Binary.AppendString(b, "hello")
	b = Binary.AppendI64(b, 2)

	// map<i32, list<i32>>
	b = Binary.AppendMapBegin(b, I32, LIST, 1)
	b = Binary.AppendI32(b, 1)
	b = Binary.AppendListBegin(b, I32, 1)
	b = Binary.AppendI32(b, 1)

	// map<list<i32>, i32>
	b = Binary.AppendMapBegin(b, LIST, I32, 1)
	b = Binary.AppendListBegin(b, I32, 1)
	b = Binary.AppendI32(b, 1)
	b = Binary.AppendI32(b, 1)

	// struct i32, list<i32>
	b = Binary.AppendFieldBegin(b, I32, 1)
	b = Binary.AppendI32(b, 1)
	b = Binary.AppendFieldBegin(b, LIST, 1)
	b = Binary.AppendListBegin(b, I32, 1)
	b = Binary.AppendI32(b, 1)
	b = Binary.AppendFieldStop(b)

	off := 0

	l, err := Binary.Skip(b[off:], BYTE)
	require.NoError(t, err)
	off += l

	l, err = Binary.Skip(b[off:], STRING)
	require.NoError(t, err)
	off += l

	l, err = Binary.Skip(b[off:], LIST) // list<i32>
	require.NoError(t, err)
	off += l

	l, err = Binary.Skip(b[off:], LIST) // list<string>
	require.NoError(t, err)
	off += l

	l, err = Binary.Skip(b[off:], LIST) // list<list<i32>>
	require.NoError(t, err)
	off += l

	l, err = Binary.Skip(b[off:], MAP) // map<i32, i64>
	require.NoError(t, err)
	off += l

	l, err = Binary.Skip(b[off:], MAP) // map<i32, string>
	require.NoError(t, err)
	off += l

	l, err = Binary.Skip(b[off:], MAP) // map<string, i64>
	require.NoError(t, err)
	off += l

	l, err = Binary.Skip(b[off:], MAP) // map<i32, list<i32>>
	require.NoError(t, err)
	off += l

	l, err = Binary.Skip(b[off:], MAP) // map<list<i32>, i32>
	require.NoError(t, err)
	off += l

	l, err = Binary.Skip(b[off:], STRUCT) // struct i32, list<i32>
	require.NoError(t, err)
	off += l

	require.Equal(t, len(b), off)

	// errDepthLimitExceeded
	b = b[:0]
	for i := 0; i < defaultRecursionDepth+1; i++ {
		b = Binary.AppendFieldBegin(b, STRUCT, 1)
	}
	_, err = Binary.Skip(b, STRUCT)
	require.Same(t, errDepthLimitExceeded, err)

	// unknown type
	_, err = Binary.Skip(b, TType(122))
	require.Error(t, err)
}

func TestNocopyWrite(t *testing.T) {
	largestr := strings.Repeat("l", nocopyWriteThreshold)
	smallstr := strings.Repeat("s", 10)

	// generate expected data
	x := BinaryProtocol{}
	expectb := make([]byte, 0, 2*x.StringLength(smallstr)+x.StringLength(largestr))
	expectb = x.AppendString(expectb, smallstr)
	expectb = x.AppendString(expectb, largestr)
	expectb = x.AppendString(expectb, largestr)
	expectb = x.AppendString(expectb, smallstr)

	// generate testing data
	i := 0
	w := &netpoll.NetpollDirectWriter{}
	b := w.Malloc(len(expectb))
	i += x.WriteStringNocopy(b[i:], w, smallstr)
	i += x.WriteStringNocopy(b[i:], w, largestr)
	i += x.WriteBinaryNocopy(b[i:], w, []byte(largestr))
	i += x.WriteStringNocopy(b[i:], w, smallstr)
	require.Equal(t, len(expectb)-i, 2*len(largestr)) // without 2*len(largestr)
	require.Equal(t, 2, w.WriteDirectN())
	require.Equal(t, expectb, w.Bytes())
}

func TestSetSpanCache(t *testing.T) {
	// initial status
	require.Nil(t, spanCache)
	// enable and init span cache
	SetSpanCache(true)
	require.NotNil(t, spanCache)
}

func BenchmarkWriteString(b *testing.B) {
	smallstr := "helloworld"
	buf := make([]byte, 4+len(smallstr))
	x := BinaryProtocol{}

	b.Run("WriteString", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			x.WriteString(buf, smallstr)
		}
	})
	b.Run("WriteStringNoCopy", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			x.WriteStringNocopy(buf, nil, smallstr)
		}
	})
}
