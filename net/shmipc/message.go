package shmipc

import (
	"encoding/binary"
)

// Message represents a protocol message that can be encoded and decoded
type Message interface {
	// Append encodes the message to the provided buffer and returns the updated buffer
	Append(buf []byte) []byte
	// Decode decodes the message from bytes
	Decode(data []byte) error
	// IsValid returns true if the message is valid
	IsValid() bool
}

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
	m.Length = uint32(headerSize + 2 + len(m.QueueFile) + 2 + len(m.BufferFile))
	m.Magic = headerMagic

	// Encode header
	buf = m.Header.Append(buf)

	// Append queue file with length prefix
	buf = appendStr(buf, m.QueueFile)

	// Append buffer file with length prefix
	buf = appendStr(buf, m.BufferFile)

	return buf
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

// IsValid returns true if the message is valid
func (m *MessageShareMemory) IsValid() bool {
	return m.Header.IsValid() && (m.Type == uint8(typeShareMemoryByFilePath) || m.Type == uint8(typeShareMemoryByMemfd))
}

// NewMessageShareMemory creates a new MessageShareMemory message
func NewMessageShareMemory(version uint8, tp messageType, queueFile, bufferFile string) *MessageShareMemory {
	return &MessageShareMemory{
		Header: Header{
			Length:  uint32(headerSize + 2 + len(queueFile) + 2 + len(bufferFile)),
			Magic:   headerMagic,
			Version: version,
			Type:    uint8(tp),
		},
		QueueFile:  queueFile,
		BufferFile: bufferFile,
	}
}

// MessageStreamClose represents a stream close notification message.
// It contains the stream ID to identify which stream is being closed.
type MessageStreamClose struct {
	Header
	StreamID uint32
}

// Append encodes the message to the provided buffer
func (m *MessageStreamClose) Append(buf []byte) []byte {
	// Update header fields
	m.Length = uint32(headerSize + 4)
	m.Magic = headerMagic

	// Encode header
	buf = m.Header.Append(buf)

	// Append stream ID using big endian
	buf = binary.BigEndian.AppendUint32(buf, m.StreamID)

	return buf
}

// Decode decodes the message from bytes
func (m *MessageStreamClose) Decode(data []byte) error {
	if len(data) < headerSize+4 {
		return ErrBufferTooShort
	}

	// Decode header
	if err := m.Header.Decode(data[:headerSize]); err != nil {
		return err
	}

	// Decode stream ID
	m.StreamID = binary.BigEndian.Uint32(data[headerSize : headerSize+4])

	return nil
}

// IsValid returns true if the message is valid
func (m *MessageStreamClose) IsValid() bool {
	return m.Header.IsValid() && m.Type == uint8(typeStreamClose)
}

// NewMessageStreamClose creates a new MessageStreamClose message
func NewMessageStreamClose(version uint8, streamID uint32) *MessageStreamClose {
	return &MessageStreamClose{
		Header: Header{
			Length:  uint32(headerSize + 4),
			Magic:   headerMagic,
			Version: version,
			Type:    uint8(typeStreamClose),
		},
		StreamID: streamID,
	}
}

// MessagePolling represents a polling notification message.
// It has no payload, just the header.
type MessagePolling struct {
	Header
}

// IsValid returns true if the message is valid
func (m *MessagePolling) IsValid() bool {
	return m.Header.IsValid() && m.Type == uint8(typePolling)
}

// NewMessagePolling creates a new MessagePolling message
func NewMessagePolling(version uint8) *MessagePolling {
	return &MessagePolling{
		Header: Header{
			Length:  uint32(headerSize),
			Magic:   headerMagic,
			Version: version,
			Type:    uint8(typePolling),
		},
	}
}

// MessageAckShareMemory represents an acknowledgment for shared memory metadata.
// It has no payload, just the header.
type MessageAckShareMemory struct {
	Header
}

// IsValid returns true if the message is valid
func (m *MessageAckShareMemory) IsValid() bool {
	return m.Header.IsValid() && m.Type == uint8(typeAckShareMemory)
}

// NewMessageAckShareMemory creates a new MessageAckShareMemory message
func NewMessageAckShareMemory(version uint8) *MessageAckShareMemory {
	return &MessageAckShareMemory{
		Header: Header{
			Length:  uint32(headerSize),
			Magic:   headerMagic,
			Version: version,
			Type:    uint8(typeAckShareMemory),
		},
	}
}

// MessageExchangeProtoVersion represents a protocol version exchange message.
// It has no payload, just the header.
type MessageExchangeProtoVersion struct {
	Header
}

// IsValid returns true if the message is valid
func (m *MessageExchangeProtoVersion) IsValid() bool {
	return m.Header.IsValid() && m.Type == uint8(typeExchangeProtoVersion)
}

// NewMessageExchangeProtoVersion creates a new MessageExchangeProtoVersion message
func NewMessageExchangeProtoVersion(version uint8) *MessageExchangeProtoVersion {
	return &MessageExchangeProtoVersion{
		Header: Header{
			Length:  uint32(headerSize),
			Magic:   headerMagic,
			Version: version,
			Type:    uint8(typeExchangeProtoVersion),
		},
	}
}

// messageRaw represents a raw message with full message bytes.
// It contains the complete message data including header and payload.
type messageRaw struct {
	Header
	Data []byte // Full message bytes including header
}

// Append encodes the message to the provided buffer
func (m *messageRaw) Append(buf []byte) []byte {
	panic("not implemented, for decoding only")
}

// Decode decodes the message from bytes
func (m *messageRaw) Decode(data []byte) error {
	if len(data) < headerSize {
		return ErrBufferTooShort
	}

	// Decode header
	if err := m.Header.Decode(data[:headerSize]); err != nil {
		return err
	}

	// Reuse existing buffer by appending data
	m.Data = append(m.Data[:0], data...)
	return nil
}

// IsValid returns true if the message is valid
func (m *messageRaw) IsValid() bool {
	return m.Header.IsValid()
}

// MessageFallbackData represents a fallback data message.
// It contains sequence ID, status, and payload data.
type MessageFallbackData struct {
	Header
	SeqID   uint32
	Status  uint32
	Payload []byte
}

// Append encodes the message to the provided buffer
func (m *MessageFallbackData) Append(buf []byte) []byte {
	// Update header fields
	m.Length = uint32(headerSize + 8 + len(m.Payload))
	m.Magic = headerMagic

	// Encode header
	buf = m.Header.Append(buf)

	// Append sequence ID and status using big endian
	buf = binary.BigEndian.AppendUint32(buf, m.SeqID)
	buf = binary.BigEndian.AppendUint32(buf, m.Status)

	// Append payload
	buf = append(buf, m.Payload...)

	return buf
}

// Decode decodes the message from bytes
func (m *MessageFallbackData) Decode(data []byte) error {
	if len(data) < headerSize+8 {
		return ErrBufferTooShort
	}

	// Decode header
	if err := m.Header.Decode(data[:headerSize]); err != nil {
		return err
	}

	// Decode sequence ID and status
	m.SeqID = binary.BigEndian.Uint32(data[headerSize : headerSize+4])
	m.Status = binary.BigEndian.Uint32(data[headerSize+4 : headerSize+8])

	// Decode payload
	payloadData := data[headerSize+8:]
	m.Payload = make([]byte, len(payloadData))
	copy(m.Payload, payloadData)

	return nil
}

// IsValid returns true if the message is valid
func (m *MessageFallbackData) IsValid() bool {
	return m.Header.IsValid() && m.Type == uint8(typeFallbackData)
}

// NewMessageFallbackData creates a new MessageFallbackData message
func NewMessageFallbackData(version uint8, seqID, status uint32, payload []byte) *MessageFallbackData {
	return &MessageFallbackData{
		Header: Header{
			Length:  uint32(headerSize + 8 + len(payload)),
			Magic:   headerMagic,
			Version: version,
			Type:    uint8(typeFallbackData),
		},
		SeqID:   seqID,
		Status:  status,
		Payload: payload,
	}
}

// MessageAckReadyRecvFD represents an acknowledgment for memfd file descriptor readiness.
// It has no payload, just the header.
type MessageAckReadyRecvFD struct {
	Header
}

// Append encodes the message to the provided buffer
func (m *MessageAckReadyRecvFD) Append(buf []byte) []byte {
	// Update header fields
	m.Length = uint32(headerSize)
	m.Magic = headerMagic

	// Encode header
	buf = m.Header.Append(buf)
	return buf
}

// Decode decodes the message from bytes
func (m *MessageAckReadyRecvFD) Decode(data []byte) error {
	if len(data) < headerSize {
		return ErrBufferTooShort
	}

	// Decode header
	return m.Header.Decode(data[:headerSize])
}

// IsValid returns true if the message is valid
func (m *MessageAckReadyRecvFD) IsValid() bool {
	return m.Header.IsValid() && m.Type == uint8(typeAckReadyRecvFD)
}

// NewMessageAckReadyRecvFD creates a new MessageAckReadyRecvFD message
func NewMessageAckReadyRecvFD(version uint8) *MessageAckReadyRecvFD {
	return &MessageAckReadyRecvFD{
		Header: Header{
			Length:  uint32(headerSize),
			Magic:   headerMagic,
			Version: version,
			Type:    uint8(typeAckReadyRecvFD),
		},
	}
}

// MessageHotRestart represents a hot restart notification message.
// It contains an epoch ID for restart coordination.
type MessageHotRestart struct {
	Header
	EpochID uint64
}

// Append encodes the message to the provided buffer
func (m *MessageHotRestart) Append(buf []byte) []byte {
	// Update header fields
	m.Length = uint32(headerSize + 8)
	m.Magic = headerMagic

	// Encode header
	buf = m.Header.Append(buf)

	// Append epoch ID using big endian
	buf = binary.BigEndian.AppendUint64(buf, m.EpochID)

	return buf
}

// Decode decodes the message from bytes
func (m *MessageHotRestart) Decode(data []byte) error {
	if len(data) < headerSize+8 {
		return ErrBufferTooShort
	}

	// Decode header
	if err := m.Header.Decode(data[:headerSize]); err != nil {
		return err
	}

	// Decode epoch ID
	m.EpochID = binary.BigEndian.Uint64(data[headerSize : headerSize+8])

	return nil
}

// IsValid returns true if the message is valid
func (m *MessageHotRestart) IsValid() bool {
	return m.Header.IsValid() && m.Type == uint8(typeHotRestart)
}

// NewMessageHotRestart creates a new MessageHotRestart message
func NewMessageHotRestart(version uint8, epochID uint64) *MessageHotRestart {
	return &MessageHotRestart{
		Header: Header{
			Length:  uint32(headerSize + 8),
			Magic:   headerMagic,
			Version: version,
			Type:    uint8(typeHotRestart),
		},
		EpochID: epochID,
	}
}

// MessageHotRestartAck represents a hot restart acknowledgment message.
// It contains an epoch ID for restart coordination.
type MessageHotRestartAck struct {
	Header
	EpochID uint64
}

// Append encodes the message to the provided buffer
func (m *MessageHotRestartAck) Append(buf []byte) []byte {
	// Update header fields
	m.Length = uint32(headerSize + 8)
	m.Magic = headerMagic

	// Encode header
	buf = m.Header.Append(buf)

	// Append epoch ID using big endian
	buf = binary.BigEndian.AppendUint64(buf, m.EpochID)

	return buf
}

// Decode decodes the message from bytes
func (m *MessageHotRestartAck) Decode(data []byte) error {
	if len(data) < headerSize+8 {
		return ErrBufferTooShort
	}

	// Decode header
	if err := m.Header.Decode(data[:headerSize]); err != nil {
		return err
	}

	// Decode epoch ID
	m.EpochID = binary.BigEndian.Uint64(data[headerSize : headerSize+8])

	return nil
}

// IsValid returns true if the message is valid
func (m *MessageHotRestartAck) IsValid() bool {
	return m.Header.IsValid() && m.Type == uint8(typeHotRestartAck)
}

// NewMessageHotRestartAck creates a new MessageHotRestartAck message
func NewMessageHotRestartAck(version uint8, epochID uint64) *MessageHotRestartAck {
	return &MessageHotRestartAck{
		Header: Header{
			Length:  uint32(headerSize + 8),
			Magic:   headerMagic,
			Version: version,
			Type:    uint8(typeHotRestartAck),
		},
		EpochID: epochID,
	}
}
