package shmipc

// MessageShareMemory represents a shared memory metadata message.
// It contains two string fields (queue and buffer paths/names) and can be used
// for both file-based (/dev/shm) and memfd-based shared memory.
type MessageShareMemory struct {
	Header
	QueueFile  string // Queue path (file-based) or name (memfd)
	BufferFile string // Buffer path (file-based) or name (memfd)
}

// Append encodes the message to the provided buffer
func (m *MessageShareMemory) Append(buf []byte) []byte {
	// Update header fields
	m.Header.Length = uint32(m.Size())
	m.Header.Magic = headerMagic

	// Encode header
	buf = m.Header.Append(buf)

	// Append queue file with length prefix
	buf = appendStr(buf, m.QueueFile)

	// Append buffer file with length prefix
	buf = appendStr(buf, m.BufferFile)

	return buf
}

// AppendByType encodes the message with a specific event type
func (m *MessageShareMemory) AppendByType(buf []byte, eventType eventType) []byte {
	m.Header.Type = uint8(eventType)
	return m.Append(buf)
}

// Decode decodes the message from bytes
func (m *MessageShareMemory) Decode(data []byte) error {
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
	bufferFile, _, err := decodeStr(data[offset:])
	if err != nil {
		return err
	}
	m.BufferFile = bufferFile

	return nil
}

// Size returns the total size of the encoded message
func (m *MessageShareMemory) Size() int {
	return headerSize + 2 + len(m.QueueFile) + 2 + len(m.BufferFile)
}

// NewMessageShareMemory creates a new MessageShareMemory message
func NewMessageShareMemory(version uint8, queueFile, bufferFile string) *MessageShareMemory {
	return &MessageShareMemory{
		Header: Header{
			Length:  uint32(headerSize + 2 + len(queueFile) + 2 + len(bufferFile)),
			Magic:   headerMagic,
			Version: version,
			// Type will be set when calling AppendByType
		},
		QueueFile:  queueFile,
		BufferFile: bufferFile,
	}
}