package thrift

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudwego/gopkg/gridbuf"
)

func TestGridBufferSkip(t *testing.T) {
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

	gbuf := gridbuf.NewReadBuffer([][]byte{b})

	var ufs []byte

	ufs, err := GridBuffer.Skip(gbuf, BYTE, ufs, true)
	require.NoError(t, err)

	ufs, err = GridBuffer.Skip(gbuf, STRING, ufs, true)
	require.NoError(t, err)

	ufs, err = GridBuffer.Skip(gbuf, LIST, ufs, true) // list<i32>
	require.NoError(t, err)

	ufs, err = GridBuffer.Skip(gbuf, LIST, ufs, true) // list<string>
	require.NoError(t, err)

	ufs, err = GridBuffer.Skip(gbuf, LIST, ufs, true) // list<list<i32>>
	require.NoError(t, err)

	ufs, err = GridBuffer.Skip(gbuf, MAP, ufs, true) // map<i32, i64>
	require.NoError(t, err)

	ufs, err = GridBuffer.Skip(gbuf, MAP, ufs, true) // map<i32, string>
	require.NoError(t, err)

	ufs, err = GridBuffer.Skip(gbuf, MAP, ufs, true) // map<string, i64>
	require.NoError(t, err)

	ufs, err = GridBuffer.Skip(gbuf, MAP, ufs, true) // map<i32, list<i32>>
	require.NoError(t, err)

	ufs, err = GridBuffer.Skip(gbuf, MAP, ufs, true) // map<list<i32>, i32>
	require.NoError(t, err)

	ufs, err = GridBuffer.Skip(gbuf, STRUCT, ufs, true) // struct i32, list<i32>
	require.NoError(t, err)

	require.Equal(t, b, ufs)

	// errDepthLimitExceeded
	b = b[:0]
	for i := 0; i < defaultRecursionDepth+1; i++ {
		b = Binary.AppendFieldBegin(b, STRUCT, 1)
	}
	gbuf = gridbuf.NewReadBuffer([][]byte{b})
	_, err = GridBuffer.Skip(gbuf, STRUCT, nil, false)
	require.Same(t, errDepthLimitExceeded, err)

	// unknown type
	gbuf = gridbuf.NewReadBuffer([][]byte{b})
	_, err = GridBuffer.Skip(gbuf, TType(122), nil, false)
	require.Error(t, err)
}
