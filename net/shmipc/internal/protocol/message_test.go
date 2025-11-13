package protocol

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageShareMemory_Encode_Decode(t *testing.T) {
	msg := NewMessageShareMemory(3, TypeShareMemoryByFilePath, "/dev/shm/queue", "/dev/shm/buffer")

	buf := msg.Append(nil)
	assert.NotEmpty(t, buf)

	var decoded MessageShareMemory
	err := decoded.Decode(buf)
	require.NoError(t, err)
	assert.Equal(t, msg.Version, decoded.Version)
	assert.Equal(t, msg.Type, decoded.Type)
	assert.Equal(t, msg.QueueFile, decoded.QueueFile)
	assert.Equal(t, msg.BufferFile, decoded.BufferFile)
	assert.True(t, decoded.IsValid())
}

func TestMessageStreamClose_Encode_Decode(t *testing.T) {
	msg := NewMessageStreamClose(3, 12345)

	buf := msg.Append(nil)
	assert.NotEmpty(t, buf)

	var decoded MessageStreamClose
	err := decoded.Decode(buf)
	require.NoError(t, err)
	assert.Equal(t, msg.Version, decoded.Version)
	assert.Equal(t, msg.StreamID, decoded.StreamID)
	assert.True(t, decoded.IsValid())
}

func TestMessagePolling(t *testing.T) {
	msg := NewMessagePolling(3)

	buf := msg.Header.Append(nil)
	assert.NotEmpty(t, buf)

	var decoded MessagePolling
	err := decoded.Header.Decode(buf)
	require.NoError(t, err)
	assert.Equal(t, msg.Version, decoded.Version)
	assert.True(t, decoded.IsValid())
}

func TestMessageAckShareMemory(t *testing.T) {
	msg := NewMessageAckShareMemory(3)
	assert.True(t, msg.IsValid())
	assert.Equal(t, uint8(TypeAckShareMemory), msg.Type)
}

func TestMessageExchangeProtoVersion(t *testing.T) {
	msg := NewMessageExchangeProtoVersion(3)
	assert.True(t, msg.IsValid())
	assert.Equal(t, uint8(TypeExchangeProtoVersion), msg.Type)
}

func TestMessageFallbackData_Encode_Decode(t *testing.T) {
	payload := []byte("test payload data")
	msg := NewMessageFallbackData(3, 42, 100, payload)

	buf := msg.Append(nil)
	assert.NotEmpty(t, buf)

	var decoded MessageFallbackData
	err := decoded.Decode(buf)
	require.NoError(t, err)
	assert.Equal(t, msg.SeqID, decoded.SeqID)
	assert.Equal(t, msg.Status, decoded.Status)
	assert.Equal(t, payload, decoded.Payload)
	assert.True(t, decoded.IsValid())
}

func TestMessageAckReadyRecvFD_Encode_Decode(t *testing.T) {
	msg := NewMessageAckReadyRecvFD(3)

	buf := msg.Append(nil)
	assert.NotEmpty(t, buf)

	var decoded MessageAckReadyRecvFD
	err := decoded.Decode(buf)
	require.NoError(t, err)
	assert.Equal(t, msg.Version, decoded.Version)
	assert.True(t, decoded.IsValid())
}

func TestMessageHotRestart_Encode_Decode(t *testing.T) {
	msg := NewMessageHotRestart(3, 9876543210)

	buf := msg.Append(nil)
	assert.NotEmpty(t, buf)

	var decoded MessageHotRestart
	err := decoded.Decode(buf)
	require.NoError(t, err)
	assert.Equal(t, msg.EpochID, decoded.EpochID)
	assert.True(t, decoded.IsValid())
}

func TestMessageHotRestartAck_Encode_Decode(t *testing.T) {
	msg := NewMessageHotRestartAck(3, 9876543210)

	buf := msg.Append(nil)
	assert.NotEmpty(t, buf)

	var decoded MessageHotRestartAck
	err := decoded.Decode(buf)
	require.NoError(t, err)
	assert.Equal(t, msg.EpochID, decoded.EpochID)
	assert.True(t, decoded.IsValid())
}

func TestMessageRaw_Decode(t *testing.T) {
	// Create a valid header message
	header := &Header{
		Length:  uint32(HeaderSize),
		Magic:   HeaderMagic,
		Version: 3,
		Type:    uint8(TypePolling),
	}
	buf := header.Append(nil)

	var raw RawMessage
	err := raw.Decode(buf)
	require.NoError(t, err)
	assert.Equal(t, header.Magic, raw.Magic)
	assert.Equal(t, buf, raw.Data)
	assert.True(t, raw.IsValid())
}
