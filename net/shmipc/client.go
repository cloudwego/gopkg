package shmipc

import (
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
	ErrInvalidMagic       = errors.New("invalid magic number")
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
	recvBuf       []byte
	version       uint8
	shmManager    SharedMemoryManager
	handshakeDone bool
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
		conn:    conn,
		sendBuf: make([]byte, 4096), // Start with 4KB, will grow as needed
		recvBuf: make([]byte, 4096), // Start with 4KB, will grow as needed
		version: 3, // Use protocol version 3 initially, will be negotiated
	}

	// Perform version and shared memory handshake
	if err := client.handshake(ctx); err != nil {
		conn.Close()
		return nil, err
	}

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
	header := Header{
		Length:  uint32(headerSize),
		Magic:   headerMagic,
		Version: c.version,
		Type:    uint8(typeExchangeProtoVersion),
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Ensure send buffer is large enough
	if len(c.sendBuf) < headerSize {
		newSize := headerSize + headerSize/2
		if newSize < 8192 {
			newSize = 8192
		}
		c.sendBuf = make([]byte, newSize)
	}

	// Send version
	header.Append(c.sendBuf[:headerSize])
	if _, err := c.conn.Write(c.sendBuf[:headerSize]); err != nil {
		return err
	}

	// Receive server version response
	_, err := c.conn.Read(c.recvBuf[:headerSize])
	if err != nil {
		return err
	}

	var responseHeader Header
	if err := responseHeader.Decode(c.recvBuf[:headerSize]); err != nil {
		return err
	}

	// Validate response
	if !responseHeader.IsValid() {
		return ErrInvalidMagic
	}
	if responseHeader.Type != uint8(typeExchangeProtoVersion) {
		return ErrVersionMismatch
	}

	serverVersion := responseHeader.Version
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
	// Generate metadata using message type
	msg := NewMessageShareMemory(c.version, c.shmManager.GetQueuePath(), c.shmManager.GetBufferPath())
	metadata := msg.AppendByType(nil, typeShareMemoryByFilePath)

	c.mu.Lock()
	defer c.mu.Unlock()

	// Ensure send buffer is large enough
	if len(c.sendBuf) < len(metadata) {
		newSize := len(metadata) + len(metadata)/2
		if newSize < 8192 {
			newSize = 8192
		}
		c.sendBuf = make([]byte, newSize)
	}

	if _, err := c.conn.Write(metadata); err != nil {
		return err
	}

	// Wait for acknowledgment
	if _, err := c.conn.Read(c.recvBuf[:headerSize]); err != nil {
		return err
	}

	var ackHeader Header
	if err := ackHeader.Decode(c.recvBuf[:headerSize]); err != nil {
		return err
	}

	if !ackHeader.IsValid() || ackHeader.Type != uint8(typeAckShareMemory) {
		return fmt.Errorf("expected ack share memory, got: type=%d", ackHeader.Type)
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
	if !c.closed.CompareAndSwap(false, true) {
		return nil // already closed
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	var lastErr error

	// Cleanup shared memory
	if c.shmManager != nil {
		if err := c.shmManager.Cleanup(); err != nil {
			lastErr = err
		}
	}

	// Close connection
	if err := c.conn.Close(); err != nil {
		lastErr = err
	}

	return lastErr
}

// Send sends a message to the server
func (c *Client) Send(msgType eventType, data []byte) error {
	if c.closed.Load() {
		return net.ErrClosed
	}

	header := Header{
		Length:  uint32(headerSize + len(data)),
		Magic:   headerMagic,
		Version: 3,
		Type:    uint8(msgType),
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Ensure send buffer is large enough, grow exponentially
	totalSize := headerSize + len(data)
	if len(c.sendBuf) < totalSize {
		// Grow buffer to at least totalSize, with 50% extra capacity for future growth
		newSize := totalSize + totalSize/2
		if newSize < 8192 { // Minimum 8KB for growth
			newSize = 8192
		}
		c.sendBuf = make([]byte, newSize)
	}

	// Encode header
	header.Append(c.sendBuf[:headerSize])

	// Copy data
	if len(data) > 0 {
		copy(c.sendBuf[headerSize:], data)
	}

	_, err := c.conn.Write(c.sendBuf[:totalSize])
	return err
}

// Receive receives a message from the server
func (c *Client) Receive() (eventType, []byte, error) {
	if c.closed.Load() {
		return 0, nil, net.ErrClosed
	}

	// Read header
	_, err := c.conn.Read(c.recvBuf[:headerSize])
	if err != nil {
		return 0, nil, err
	}

	var header Header
	if err := header.Decode(c.recvBuf[:headerSize]); err != nil {
		return 0, nil, err
	}

	// Validate magic number
	if !header.IsValid() {
		return 0, nil, ErrInvalidMagic
	}

	// Read payload if any
	payloadSize := int(header.Length) - headerSize
	var payload []byte
	if payloadSize > 0 {
		if len(c.recvBuf) < payloadSize {
			// Grow buffer exponentially for large payloads
			newSize := payloadSize + payloadSize/2
			if newSize < 8192 { // Minimum 8KB for growth
				newSize = 8192
			}
			c.recvBuf = make([]byte, newSize)
		}
		_, err := c.conn.Read(c.recvBuf[:payloadSize])
		if err != nil {
			return 0, nil, err
		}
		payload = c.recvBuf[:payloadSize]
	}

	return eventType(header.Type), payload, nil
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