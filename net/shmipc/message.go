package shmipc

import (
	"encoding/binary"
)

// MessageShareMemoryByMemFD represents a shared memory metadata message for MemFD.
// After sending this message, the sender must wait for typeAckReadyRecvFD response,
// then send the actual file descriptors using SCM_RIGHTS ancillary data.
type MessageShareMemoryByMemFD struct {
	Header
	QueueFile  string // MemFD name for queue
	BufferFile string // MemFD name for buffer
}

// Append encodes the MessageShareMemoryByMemFD message to the provided buffer
func (m *MessageShareMemoryByMemFD) Append(buf []byte) []byte {
	// Update header fields
	m.Header.Length = uint32(m.Size())
	m.Header.Magic = headerMagic
	m.Header.Type = uint8(typeShareMemoryByMemfd)

	// Encode header
	buf = m.Header.Append(buf)

	// Append queue file with length prefix
	buf = appendStr(buf, m.QueueFile)

	// Append buffer file with length prefix
	buf = appendStr(buf, m.BufferFile)

	return buf
}

// Decode decodes the MessageShareMemoryByMemFD message from bytes
func (m *MessageShareMemoryByMemFD) Decode(data []byte) error {
	if len(data) < headerSize {
		return ErrBufferTooShort
	}

	// Decode header
	if err := m.Header.Decode(data[:headerSize]); err != nil {
		return err
	}

	offset := headerSize

	// Decode queue file
	queueFile, n, err := decodeStr(data[offset:])
	if err != nil {
		return err
	}
	m.QueueFile = queueFile
	offset += n

	// Decode buffer file
	bufferFile, n, err := decodeStr(data[offset:])
	if err != nil {
		return err
	}
	m.BufferFile = bufferFile

	return nil
}

// Size returns the total size of the encoded message
func (m *MessageShareMemoryByMemFD) Size() int {
	return headerSize + 2 + len(m.QueueFile) + 2 + len(m.BufferFile)
}

// NewMessageShareMemoryByMemFD creates a new MessageShareMemoryByMemFD message
func NewMessageShareMemoryByMemFD(version uint8, queueFile, bufferFile string) *MessageShareMemoryByMemFD {
	return &MessageShareMemoryByMemFD{
		Header: Header{
			Length:  uint32(headerSize + 2 + len(queueFile) + 2 + len(bufferFile)),
			Magic:   headerMagic,
			Version: version,
			Type:    uint8(typeShareMemoryByMemfd),
		},
		QueueFile:  queueFile,
		BufferFile: bufferFile,
	}
}