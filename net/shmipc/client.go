package shmipc

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrVersionMismatch    = errors.New("protocol version mismatch")
	ErrUnsupportedVersion = errors.New("unsupported protocol version")
	ErrInvalid            = errors.New("invalid type or magic number")
)

const (
	MinSupportedVersion = 3
	MaxSupportedVersion = 3
)

// Client represents a shared memory IPC client
type Client struct {
	conn          net.Conn
	mu            sync.RWMutex
	closed        atomic.Bool
	sendBuf       []byte
	reader        *bufio.Reader
	version       uint8
	shmManager    SharedMemoryManager
	handshakeDone bool
	nextStreamID  atomic.Uint32

	// Stream management
	streams   map[uint32]*Stream
	streamsMu sync.RWMutex

	// Queue notification channel
	queueNotifyCh chan struct{}
}

// ClientConfig contains client configuration
type ClientConfig struct {
	SocketPath string
	Timeout    time.Duration
}

// NewClient creates a new IPC client
func NewClient(config ClientConfig) (*Client, error) {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	conn, err := dialUnixSocket(ctx, config.SocketPath)
	if err != nil {
		return nil, err
	}

	client := &Client{
		conn:          conn,
		sendBuf:       make([]byte, 16<<10),              // Start with 16KB, will grow as needed
		reader:        bufio.NewReaderSize(conn, 16<<10), // 16KB read buffer
		version:       3,                                 // Use protocol version 3 initially, will be negotiated
		streams:       make(map[uint32]*Stream),
		queueNotifyCh: make(chan struct{}, 1),
	}
	// Client uses odd stream IDs starting from 1
	client.nextStreamID.Store(1)

	// Perform version and shared memory handshake
	if err := client.handshake(ctx); err != nil {
		conn.Close()
		return nil, err
	}

	// Start background goroutines for receiving and queue processing
	go client.recvLoop()
	go client.queueLoop()

	return client, nil
}

// dialUnixSocket connects to a Unix domain socket
func dialUnixSocket(ctx context.Context, path string) (net.Conn, error) {
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "unix", path)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

// handshake performs protocol version negotiation and shared memory setup
func (c *Client) handshake(ctx context.Context) error {
	// Validate client version
	if c.version < MinSupportedVersion || c.version > MaxSupportedVersion {
		return ErrUnsupportedVersion
	}

	// Step 1: Version negotiation
	if err := c.negotiateVersion(); err != nil {
		return err
	}

	// Step 2: Initialize shared memory manager based on negotiated version
	if err := c.initializeSharedMemory(); err != nil {
		return err
	}

	// Step 3: Exchange shared memory metadata
	if err := c.exchangeSharedMemoryMetadata(); err != nil {
		return err
	}

	c.handshakeDone = true
	return nil
}

// negotiateVersion handles the protocol version exchange
func (c *Client) negotiateVersion() error {
	// Send version message
	msg := NewMessageExchangeProtoVersion(c.version)
	if err := c.SendMessage(msg); err != nil {
		return err
	}

	// Receive server version response
	var responseMsg MessageExchangeProtoVersion
	if err := c.recvMessage(&responseMsg); err != nil {
		return err
	}

	// Validate response
	if !responseMsg.IsValid() {
		return ErrInvalid
	}

	serverVersion := responseMsg.Version
	if serverVersion < MinSupportedVersion || serverVersion > MaxSupportedVersion {
		return ErrUnsupportedVersion
	}

	// Choose minimum version for compatibility
	if serverVersion < c.version {
		c.version = serverVersion
	}

	return nil
}

// initializeSharedMemory creates the appropriate shared memory manager
func (c *Client) initializeSharedMemory() error {
	// For version 3, default to MemFD if available, otherwise fallback to file
	c.shmManager = NewMemFDBasedShmManager()
	if err := c.shmManager.Initialize(); err != nil {
		// Fallback to file-based if MemFD fails
		c.shmManager = NewFileBasedShmManager()
	}

	return c.shmManager.Initialize()
}

// exchangeSharedMemoryMetadata handles the shared memory metadata exchange
func (c *Client) exchangeSharedMemoryMetadata() error {
	if c.shmManager.GetType() == MemMapTypeDevShmFile {
		return c.exchangeFilePathMetadata()
	}
	return c.exchangeMemFDMetadata()
}

// exchangeFilePathMetadata handles file-based shared memory metadata exchange
func (c *Client) exchangeFilePathMetadata() error {
	// Generate and send metadata message
	msg := NewMessageShareMemory(c.version, typeShareMemoryByFilePath,
		c.shmManager.GetQueuePath(), c.shmManager.GetBufferPath())
	if err := c.SendMessage(msg); err != nil {
		return err
	}

	// Wait for acknowledgment
	var ackMsg MessageAckShareMemory
	if err := c.recvMessage(&ackMsg); err != nil {
		return err
	}

	if !ackMsg.IsValid() {
		return ErrInvalid
	}

	return nil
}

// exchangeMemFDMetadata handles MemFD-based shared memory metadata exchange
func (c *Client) exchangeMemFDMetadata() error {
	memfdManager, ok := c.shmManager.(*MemFDBasedShmManager)
	if !ok {
		return fmt.Errorf("shared memory manager is not MemFD-based")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Send metadata and file descriptors
	err := memfdManager.SendMetadataAndFDs(c.conn, c.version)
	if err != nil {
		return err
	}

	return nil
}

// Close closes the client connection and cleans up shared memory
func (c *Client) Close() error {
	return c.closeWithError(nil)
}

// closeWithError closes the client connection with an optional error
func (c *Client) closeWithError(err error) error {
	if !c.closed.CompareAndSwap(false, true) {
		return nil // already closed
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	var lastErr error

	// Use provided error or default close error
	if err != nil {
		lastErr = err
	} else {
		lastErr = net.ErrClosed
	}

	// Cleanup shared memory
	if c.shmManager != nil {
		if cleanupErr := c.shmManager.Cleanup(); cleanupErr != nil {
			// Prioritize the original error if it exists
			if err == nil {
				lastErr = cleanupErr
			}
		}
	}

	// Close connection
	if closeErr := c.conn.Close(); closeErr != nil {
		// Prioritize the original error if it exists
		if err == nil {
			lastErr = closeErr
		}
	}

	return lastErr
}

// OpenStream creates a new stream for bidirectional communication
func (c *Client) OpenStream() (*Stream, error) {
	if c.closed.Load() {
		return nil, net.ErrClosed
	}

	if !c.handshakeDone {
		return nil, errors.New("handshake not completed")
	}

	// Allocate new stream ID (odd numbers for client, increment by 2)
	id := c.nextStreamID.Add(2) - 2

	stream := newStream(c, id)

	// Register stream
	c.streamsMu.Lock()
	c.streams[id] = stream
	c.streamsMu.Unlock()

	return stream, nil
}

// registerStream registers a stream in the client's stream map
func (c *Client) registerStream(s *Stream) {
	c.streamsMu.Lock()
	c.streams[s.id] = s
	c.streamsMu.Unlock()
}

// unregisterStream removes a stream from the client's stream map
func (c *Client) unregisterStream(id uint32) {
	c.streamsMu.Lock()
	delete(c.streams, id)
	c.streamsMu.Unlock()
}

// getStream retrieves a stream by ID
func (c *Client) getStream(id uint32) *Stream {
	c.streamsMu.RLock()
	stream := c.streams[id]
	c.streamsMu.RUnlock()
	return stream
}

// SendMessage sends a message to the server
func (c *Client) SendMessage(m Message) error {
	if c.closed.Load() {
		return net.ErrClosed
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Encode message directly into send buffer, growing it as needed
	c.sendBuf = m.Append(c.sendBuf[:0])

	_, err := c.conn.Write(c.sendBuf)
	return err
}

// recvMessage receives a message from the server and decodes it into the provided Message
func (c *Client) recvMessage(m Message) error {
	if c.closed.Load() {
		return net.ErrClosed
	}

	// Read header first
	headerBytes, err := c.reader.Peek(headerSize)
	if err != nil {
		return err
	}

	var header Header
	if err := header.Decode(headerBytes); err != nil {
		return err
	}

	if !header.IsValid() {
		return ErrInvalid
	}

	// Read full message
	messageLen := int(header.Length)

	// If message fits in bufio buffer, use Peek for the whole message
	if messageLen <= c.reader.Size() {
		fullMessage, err := c.reader.Peek(messageLen)
		if err != nil {
			return err
		}
		// Decode message directly from peeked data
		err = m.Decode(fullMessage)
		if err != nil {
			return err
		}
		// Discard the peeked data
		_, err = c.reader.Discard(messageLen)
		return err
	}

	// Message is larger than buffer, need to allocate and read
	messageBytes := make([]byte, messageLen)
	_, err = c.reader.Read(messageBytes)
	if err != nil {
		return err
	}

	// Decode message
	return m.Decode(messageBytes)
}

// notifyPeer sends a notification to the peer when data is ready in the queue
func (c *Client) notifyPeer() error {
	queueMgr := c.shmManager.GetQueueManager()
	sendQueue := queueMgr.GetSendQueue()

	// Only send notification if peer needs to be notified
	if !sendQueue.MarkWorking() {
		return nil
	}

	// Send polling event to wake up peer
	msg := NewMessagePolling(c.version)
	return c.SendMessage(msg)
}

// recvLoop receives events from the Unix domain socket
func (c *Client) recvLoop() {
	var rawMsg messageRaw
	for {
		if c.closed.Load() {
			return
		}

		// Receive raw message
		if err := c.recvMessage(&rawMsg); err != nil {
			if c.closed.Load() {
				return
			}
			// Handle receive error by closing client
			c.closeWithError(err)
			return
		}

		switch messageType(rawMsg.Type) {
		case typePolling:
			// Handle typePolling event - notify queueLoop
			select {
			case c.queueNotifyCh <- struct{}{}:
			default:
				// Channel already has notification, skip
			}
		case typeStreamClose:
			// Handle stream close notification
			c.handleStreamClose(rawMsg.Data)
		}
	}
}

// handleStreamClose handles incoming stream close notifications
func (c *Client) handleStreamClose(messageBytes []byte) {
	// Decode the stream close message
	var msg MessageStreamClose
	if err := msg.Decode(messageBytes); err != nil {
		// Invalid message, ignore
		return
	}

	// Find and close the stream
	stream := c.getStream(msg.StreamID)
	if stream != nil {
		// Close the stream (this will also unregister it)
		stream.Close()
	}
}

// queueLoop pops from queue and dispatches to streams
func (c *Client) queueLoop() {
	queueMgr := c.shmManager.GetQueueManager()
	bufferMgr := c.shmManager.GetBufferManager()
	recvQueue := queueMgr.GetRecvQueue()

	// Use a ticker instead of time.After to avoid GC pressure
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	for {
		if c.closed.Load() {
			return
		}

		// Try to pop all available elements from queue
		for {
			elem, err := recvQueue.Pop()
			if err != nil {
				// Queue is empty, break inner loop
				break
			}

			// Find the stream for this element
			stream := c.getStream(elem.StreamID)
			if stream == nil {
				// No stream for this ID, read and recycle buffer
				slice, err := bufferMgr.ReadBuffer(elem.Offset)
				if err != nil {
					continue
				}
				bufferMgr.RecycleBuffer(slice)
				continue
			}

			// Deliver element to stream
			stream.deliverElement(elem)
		}

		// Mark not working after consuming all elements
		recvQueue.MarkNotWorking()

		// Wait for notification from recvLoop
		select {
		case <-c.queueNotifyCh:
			// Got notification, continue to pop from queue
		case <-ticker.C:
			// Timeout, check queue again (in case notification was missed)
		}
	}
}

// IsClosed returns whether the client is closed
func (c *Client) IsClosed() bool {
	return c.closed.Load()
}

// GetVersion returns the negotiated protocol version
func (c *Client) GetVersion() uint8 {
	return c.version
}

// GetSharedMemoryManager returns the shared memory manager
func (c *Client) GetSharedMemoryManager() SharedMemoryManager {
	return c.shmManager
}

// IsHandshakeDone returns whether the handshake is completed
func (c *Client) IsHandshakeDone() bool {
	return c.handshakeDone
}
