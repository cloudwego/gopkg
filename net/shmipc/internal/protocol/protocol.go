package protocol

import (
	"encoding/binary"
	"errors"
)

const (
	HeaderMagic = 0x7758
	HeaderSize  = 4 + 2 + 1 + 1 // len(4) + magic(2) + version(1) + type(1)
)

var (
	ErrBufferTooShort = errors.New("buffer too short")
)

type MessageType uint8

const (
	// TypeShareMemoryByFilePath is deprecated but kept for fallback compatibility
	TypeShareMemoryByFilePath MessageType = 0
	TypePolling               MessageType = 1
	TypeStreamClose           MessageType = 2
	TypeFallbackData          MessageType = 3
	TypeExchangeProtoVersion  MessageType = 4
	TypeShareMemoryByMemfd    MessageType = 5 // >= version 3
	TypeAckShareMemory        MessageType = 6
	TypeAckReadyRecvFD        MessageType = 7
	TypeHotRestart            MessageType = 8
	TypeHotRestartAck         MessageType = 9
)

// Header represents the shared memory IPC packet header
type Header struct {
	Length  uint32 // 4 byte - Total message length including header
	Magic   uint16 // 2 byte - Protocol identifier (0x7758)
	Version uint8  // 1 byte - Protocol version
	Type    uint8  // 1 byte - Message type
}

// Append encodes the header to the provided buffer using big endian
func (h *Header) Append(buf []byte) []byte {
	// Append length and magic using binary.BigEndian.Append methods
	buf = binary.BigEndian.AppendUint32(buf, h.Length)
	buf = binary.BigEndian.AppendUint16(buf, h.Magic)

	// Append version and type
	return append(buf, h.Version, h.Type)
}

// Decode decodes the header from bytes using big endian
func (h *Header) Decode(data []byte) error {
	if len(data) < HeaderSize {
		return ErrBufferTooShort
	}
	h.Length = binary.BigEndian.Uint32(data[0:4])
	h.Magic = binary.BigEndian.Uint16(data[4:6])
	h.Version = data[6]
	h.Type = data[7]
	return nil
}

// IsValid checks if the header has a valid magic number
func (h *Header) IsValid() bool {
	return h.Magic == HeaderMagic
}

// appendStr appends a string to the buffer with length prefix using big endian
// Panics if string length exceeds max uint16
func appendStr(buf []byte, s string) []byte {
	slen := len(s)
	if slen > int(^uint16(0)) {
		panic("string length exceeds uint16 max")
	}
	buf = binary.BigEndian.AppendUint16(buf, uint16(slen))
	buf = append(buf, s...)
	return buf
}

// decodeStr decodes a length-prefixed string from the buffer using big endian
// Returns the decoded string, number of bytes consumed, and error if buffer is too short
func decodeStr(data []byte) (string, int, error) {
	if len(data) < 2 {
		return "", 0, ErrBufferTooShort
	}
	slen := int(binary.BigEndian.Uint16(data[0:2]))
	if len(data) < 2+slen {
		return "", 0, ErrBufferTooShort
	}
	return string(data[2 : 2+slen]), 2 + slen, nil
}
