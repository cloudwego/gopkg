package thrift

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBinaryWriter(t *testing.T) {
	w := NewBinaryWriterSize(defaultBinaryWriterBufferSize * 2)
	x := BinaryProtocol{}

	b := x.AppendMessageBegin(nil, "hello", 1, 2)
	w.WriteMessageBegin("hello", 1, 2)
	require.Equal(t, b, w.Bytes())

	b = x.AppendFieldBegin(b, 3, 4)
	w.WriteFieldBegin(3, 4)
	require.Equal(t, b, w.Bytes())

	b = x.AppendFieldStop(b)
	w.WriteFieldStop()
	require.Equal(t, b, w.Bytes())

	b = x.AppendMapBegin(b, 5, 6, 7)
	w.WriteMapBegin(5, 6, 7)
	require.Equal(t, b, w.Bytes())

	b = x.AppendListBegin(b, 8, 9)
	w.WriteListBegin(8, 9)
	require.Equal(t, b, w.Bytes())

	b = x.AppendSetBegin(b, 10, 11)
	w.WriteSetBegin(10, 11)
	require.Equal(t, b, w.Bytes())

	b = x.AppendBinary(b, []byte("12"))
	w.WriteBinary([]byte("12"))
	require.Equal(t, b, w.Bytes())

	b = x.AppendString(b, "13")
	w.WriteString("13")
	require.Equal(t, b, w.Bytes())

	b = x.AppendBool(b, true)
	b = x.AppendBool(b, false)
	w.WriteBool(true)
	w.WriteBool(false)
	require.Equal(t, b, w.Bytes())

	b = x.AppendByte(b, 14)
	w.WriteByte(14)
	require.Equal(t, b, w.Bytes())

	b = x.AppendI16(b, 15)
	w.WriteI16(15)
	require.Equal(t, b, w.Bytes())

	b = x.AppendI32(b, 16)
	w.WriteI32(16)
	require.Equal(t, b, w.Bytes())

	b = x.AppendI64(b, 17)
	w.WriteI64(17)
	require.Equal(t, b, w.Bytes())

	b = x.AppendDouble(b, 18.5)
	w.WriteDouble(18.5)
	require.Equal(t, b, w.Bytes())
}
