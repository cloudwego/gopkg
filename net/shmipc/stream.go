package shmipc

import (
	"errors"
	"net"
	"sync/atomic"

	"github.com/cloudwego/gopkg/net/shmipc/internal/protocol"
)

var (
	ErrStreamClosed = errors.New("stream closed")
)

// Stream represents a logical stream within a shared memory IPC session
type Stream struct {
	id      uint32
	client  *Client
	closed  atomic.Bool
	recvCh  chan QueueElement
	closeCh chan struct{}

	// Partial read state
	currentBuf    *Buffer // Current buffer being read
	currentOffset int     // Offset within current buffer's data
}

// newStream creates a new stream with the given ID
func newStream(client *Client, id uint32) *Stream {
	return &Stream{
		id:      id,
		client:  client,
		recvCh:  make(chan QueueElement, 16), // Buffer for received elements
		closeCh: make(chan struct{}),
	}
}

// deliverElement delivers a queue element to this stream
func (s *Stream) deliverElement(elem QueueElement) {
	// Check if stream is closed first
	if s.closed.Load() {
		// Stream closed, recycle buffer
		bufferMgr := s.client.shmManager.GetBufferManager()
		buf, err := bufferMgr.ReadBuffer(elem.Offset)
		if err != nil {
			return
		}
		recycleBufferChain(bufferMgr, buf)
		return
	}

	// Try to deliver to stream
	select {
	case s.recvCh <- elem:
		// Element delivered
	case <-s.closeCh:
		// Stream closed during delivery, recycle buffer
		bufferMgr := s.client.shmManager.GetBufferManager()
		buf, err := bufferMgr.ReadBuffer(elem.Offset)
		if err != nil {
			return
		}
		recycleBufferChain(bufferMgr, buf)
	default:
		// Channel full, this shouldn't happen with proper flow control
		// TODO: handle backpressure
	}
}

// Read reads data from the stream's receive queue
func (s *Stream) Read(p []byte) (n int, err error) {
	if s.closed.Load() {
		return 0, ErrStreamClosed
	}
	bufferMgr := s.client.shmManager.GetBufferManager()

	// If we have a current buffer from a previous partial read, continue from there
	if s.currentBuf != nil {
		return s.readFromCurrentBuffer(p, bufferMgr)
	}

	// Wait for element from queueLoop
	select {
	case elem := <-s.recvCh:
		// Read buffer from shared memory (returns head of linked list)
		buf, err := bufferMgr.ReadBuffer(elem.Offset)
		if err != nil {
			return 0, err
		}

		s.currentBuf = buf
		s.currentOffset = 0
		return s.readFromCurrentBuffer(p, bufferMgr)
	case <-s.closeCh:
		return 0, ErrStreamClosed
	}
}

// readFromCurrentBuffer reads data from the current buffer chain
func (s *Stream) readFromCurrentBuffer(p []byte, bufferMgr *BufferManager) (int, error) {
	copied := 0
	for s.currentBuf != nil && copied < len(p) {
		// Copy from current position to end of valid data
		data := s.currentBuf.Data()
		n := copy(p[copied:], data[s.currentOffset:])
		copied += n
		s.currentOffset += n

		// If we've consumed all data in this buffer, move to next
		if s.currentOffset >= len(data) {
			next := s.currentBuf.next
			bufferMgr.RecycleBuffer(s.currentBuf)
			s.currentBuf = next
			s.currentOffset = 0
		}
	}
	return copied, nil
}

// Write writes data to the stream's send queue
// For large data, it allocates multiple buffers and chains them
func (s *Stream) Write(p []byte) (n int, err error) {
	if s.closed.Load() {
		return 0, ErrStreamClosed
	}
	if s.client.closed.Load() {
		return 0, net.ErrClosed
	}
	if len(p) == 0 {
		return 0, nil
	}

	queueMgr := s.client.shmManager.GetQueueManager()
	bufferMgr := s.client.shmManager.GetBufferManager()

	// Allocate buffer chain and copy data
	head, err := allocBufferChain(bufferMgr, p)
	if err != nil {
		return 0, err
	}

	// Push element to send queue with stream ID (only head offset)
	elem := QueueElement{
		StreamID: s.id,
		Offset:   head.Offset(),
		Status:   0, // TODO: proper status management
	}

	if err := queueMgr.GetSendQueue().Put(elem); err != nil {
		recycleBufferChain(bufferMgr, head)
		return 0, err
	}

	// Notify peer that data is ready
	if err := s.client.notifyPeer(); err != nil {
		return len(p), err
	}

	return len(p), nil
}

// Close closes the stream
func (s *Stream) Close() error {
	if !s.closed.CompareAndSwap(false, true) {
		return nil // already closed
	}

	// Send close notification to peer
	s.sendCloseNotification()

	// Unregister from client first - prevents new deliverElement calls
	s.client.unregisterStream(s)

	// Close closeCh to signal to any in-flight deliverElement and Read calls
	close(s.closeCh)

	bufferMgr := s.client.shmManager.GetBufferManager()

	// Clean up current buffer chain from partial reads
	if s.currentBuf != nil {
		recycleBufferChain(bufferMgr, s.currentBuf)
		s.currentBuf = nil
		s.currentOffset = 0
	}

	// Drain and recycle any remaining elements
	// Note: We don't close recvCh to avoid panic if deliverElement is still executing
	for {
		select {
		case elem := <-s.recvCh:
			buf, err := bufferMgr.ReadBuffer(elem.Offset)
			if err != nil {
				continue
			}
			recycleBufferChain(bufferMgr, buf)
		default:
			// Channel drained, let it be garbage collected
			return nil
		}
	}
}

// ID returns the stream ID
func (s *Stream) ID() uint32 {
	return s.id
}

// IsClosed returns whether the stream is closed
func (s *Stream) IsClosed() bool {
	return s.closed.Load()
}

// sendCloseNotification sends a stream close notification to the peer
func (s *Stream) sendCloseNotification() {
	// Create stream close message
	msg := protocol.NewMessageStreamClose(s.client.version, s.id)

	// Send message to peer (ignore errors as we're closing anyway)
	s.client.sendMessage(msg)
}
