package shmipc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeader_Encode_Decode(t *testing.T) {
	h := &Header{
		Length:  100,
		Magic:   headerMagic,
		Version: 3,
		Type:    uint8(typePolling),
	}

	buf := h.Append(nil)
	assert.Len(t, buf, headerSize)

	var decoded Header
	err := decoded.Decode(buf)
	require.NoError(t, err)
	assert.Equal(t, h.Length, decoded.Length)
	assert.Equal(t, h.Magic, decoded.Magic)
	assert.Equal(t, h.Version, decoded.Version)
	assert.Equal(t, h.Type, decoded.Type)
}

func TestHeader_IsValid(t *testing.T) {
	tests := []struct {
		name  string
		magic uint16
		valid bool
	}{
		{"valid magic", headerMagic, true},
		{"invalid magic", 0x0000, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Header{Magic: tt.magic}
			assert.Equal(t, tt.valid, h.IsValid())
		})
	}
}

func TestHeader_Decode_Short_Buffer(t *testing.T) {
	var h Header
	err := h.Decode([]byte{1, 2, 3})
	assert.ErrorIs(t, err, ErrBufferTooShort)
}

func TestAppendStr(t *testing.T) {
	tests := []struct {
		input string
	}{
		{""},
		{"hello"},
		{"test_string_123"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			buf := appendStr(nil, tt.input)
			decoded, n, err := decodeStr(buf)
			require.NoError(t, err)
			assert.Equal(t, tt.input, decoded)
			assert.Equal(t, len(buf), n)
		})
	}
}

func TestDecodeStr_Short_Buffer(t *testing.T) {
	_, _, err := decodeStr([]byte{0x00})
	assert.ErrorIs(t, err, ErrBufferTooShort)

	_, _, err = decodeStr([]byte{0x00, 0x05, 0x01, 0x02})
	assert.ErrorIs(t, err, ErrBufferTooShort)
}
