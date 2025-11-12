package shmipc

import (
	"errors"
	"net"
	"sync/atomic"
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
		slice, err := bufferMgr.ReadBufferSlice(elem.Offset)
		if err != nil {
			return
		}
		bufferMgr.RecycleBuffer(slice)
		return
	}

	// Try to deliver to stream
	select {
	case s.recvCh <- elem:
		// Element delivered
	case <-s.closeCh:
		// Stream closed during delivery, recycle buffer
		bufferMgr := s.client.shmManager.GetBufferManager()
		slice, err := bufferMgr.ReadBufferSlice(elem.Offset)
		if err != nil {
			return
		}
		bufferMgr.RecycleBuffer(slice)
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

	if s.client.closed.Load() {
		return 0, net.ErrClosed
	}

	if !s.client.handshakeDone {
		return 0, errors.New("handshake not completed")
	}

	bufferMgr := s.client.shmManager.GetBufferManager()

	// Wait for element from queueLoop
	select {
	case elem := <-s.recvCh:
		// Read buffer slice from shared memory
		slice, err := bufferMgr.ReadBufferSlice(elem.Offset)
		if err != nil {
			return 0, err
		}

		// Copy data to p
		data := slice.Data()
		n = copy(p, data)

		// Release buffer
		bufferMgr.RecycleBuffer(slice)

		return n, nil
	case <-s.closeCh:
		return 0, ErrStreamClosed
	}
}

// Write writes data to the stream's send queue
func (s *Stream) Write(p []byte) (n int, err error) {
	if s.closed.Load() {
		return 0, ErrStreamClosed
	}

	if s.client.closed.Load() {
		return 0, net.ErrClosed
	}

	if !s.client.handshakeDone {
		return 0, errors.New("handshake not completed")
	}

	queueMgr := s.client.shmManager.GetQueueManager()
	bufferMgr := s.client.shmManager.GetBufferManager()

	// Allocate buffer
	slice, err := bufferMgr.AllocBuffer(uint32(len(p)))
	if err != nil {
		return 0, err
	}

	// Copy data to buffer
	copy(slice.Data(), p)
	slice.SetLen(uint32(len(p)))

	// Push element to send queue with stream ID
	elem := QueueElement{
		StreamID: s.id,
		Offset:   slice.Offset(),
		Status:   0, // TODO: proper status management
	}

	if err := queueMgr.GetSendQueue().Put(elem); err != nil {
		bufferMgr.RecycleBuffer(slice)
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
	s.client.unregisterStream(s.id)

	// Close closeCh to signal to any in-flight deliverElement and Read calls
	close(s.closeCh)

	// Drain and recycle any remaining elements
	// Note: We don't close recvCh to avoid panic if deliverElement is still executing
	bufferMgr := s.client.shmManager.GetBufferManager()
	for {
		select {
		case elem := <-s.recvCh:
			slice, err := bufferMgr.ReadBufferSlice(elem.Offset)
			if err != nil {
				continue
			}
			bufferMgr.RecycleBuffer(slice)
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
	msg := NewMessageStreamClose(s.client.version, s.id)

	// Send message to peer (ignore errors as we're closing anyway)
	s.client.SendMessage(msg)
}
