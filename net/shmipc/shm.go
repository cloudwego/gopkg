package shmipc

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

var (
	ErrShmNotInitialized = errors.New("shared memory not initialized")
	ErrUnsupportedMemMap = errors.New("unsupported memory mapping type")
)

// MemMapType specifies the type of shared memory mapping
type MemMapType uint8

const (
	// MemMapTypeDevShmFile maps shared memory to /dev/shm (tmpfs)
	MemMapTypeDevShmFile MemMapType = 0
	// MemMapTypeMemFd maps shared memory to memfd (Linux v3.17+)
	MemMapTypeMemFd MemMapType = 1
)

// SharedMemoryManager defines the interface for shared memory operations
type SharedMemoryManager interface {
	// Initialize sets up the shared memory regions
	Initialize() error

	// GenerateMetadata creates the metadata payload for sharing with peer
	GenerateMetadata(version uint8, eventType eventType) ([]byte, error)

	// ParseMetadata processes received metadata from peer
	ParseMetadata(data []byte) error

	// Cleanup removes shared memory resources
	Cleanup() error

	// GetQueuePath returns the queue shared memory path
	GetQueuePath() string

	// GetBufferPath returns the buffer shared memory path
	GetBufferPath() string

	// GetType returns the memory mapping type
	GetType() MemMapType
}

// FileBasedShmManager implements SharedMemoryManager for file-based shared memory (fallback)
type FileBasedShmManager struct {
	queuePath  string
	bufferPath string
	initialized bool
}

// NewFileBasedShmManager creates a new file-based shared memory manager
func NewFileBasedShmManager() *FileBasedShmManager {
	pid := os.Getpid()
	timestamp := strconv.Itoa(int(time.Now().UnixNano() % 1000000))

	return &FileBasedShmManager{
		queuePath:  fmt.Sprintf("/dev/shm/shmipc_queue_%d_%s", pid, timestamp),
		bufferPath: fmt.Sprintf("/dev/shm/shmipc_buffer_%d_%s", pid, timestamp),
		initialized: false,
	}
}

// Initialize sets up the shared memory files
func (f *FileBasedShmManager) Initialize() error {
	// Create queue shared memory file
	if err := f.createSharedMemoryFile(f.queuePath, 32*1024); err != nil {
		return fmt.Errorf("failed to create queue shared memory: %w", err)
	}

	// Create buffer shared memory file
	if err := f.createSharedMemoryFile(f.bufferPath, 32*1024*1024); err != nil {
		os.Remove(f.queuePath) // cleanup
		return fmt.Errorf("failed to create buffer shared memory: %w", err)
	}

	f.initialized = true
	return nil
}

// createSharedMemoryFile creates a shared memory file with the given size
func (f *FileBasedShmManager) createSharedMemoryFile(path string, size int) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// Set file size
	if err := file.Truncate(int64(size)); err != nil {
		return err
	}

	return nil
}

// GenerateMetadata creates metadata payload for typeShareMemoryByFilePath
func (f *FileBasedShmManager) GenerateMetadata(version uint8, eventType eventType) ([]byte, error) {
	if !f.initialized {
		return nil, ErrShmNotInitialized
	}

	// Layout: header + queuePathLen(2) + queuePath + bufferPathLen(2) + bufferPath
	queuePathLen := len(f.queuePath)
	bufferPathLen := len(f.bufferPath)
	totalSize := headerSize + 2 + queuePathLen + 2 + bufferPathLen

	data := make([]byte, totalSize)
	offset := headerSize

	// Queue path length and path
	binary.BigEndian.PutUint16(data[offset:offset+2], uint16(queuePathLen))
	offset += 2
	copy(data[offset:offset+queuePathLen], f.queuePath)
	offset += queuePathLen

	// Buffer path length and path
	binary.BigEndian.PutUint16(data[offset:offset+2], uint16(bufferPathLen))
	offset += 2
	copy(data[offset:offset+bufferPathLen], f.bufferPath)

	// Encode header
	header := Header{
		Length:  uint32(totalSize),
		Magic:   headerMagic,
		Version: version,
		Type:    uint8(eventType),
	}
	header.Append(data[:headerSize])

	return data, nil
}

// ParseMetadata processes received metadata from peer
func (f *FileBasedShmManager) ParseMetadata(data []byte) error {
	if len(data) < headerSize {
		return ErrBufferTooShort
	}

	offset := headerSize

	// Read queue path length
	if len(data) < offset+2 {
		return ErrBufferTooShort
	}
	queuePathLen := int(binary.BigEndian.Uint16(data[offset : offset+2]))
	offset += 2

	// Read queue path
	if len(data) < offset+queuePathLen {
		return ErrBufferTooShort
	}
	f.queuePath = string(data[offset : offset+queuePathLen])
	offset += queuePathLen

	// Read buffer path length
	if len(data) < offset+2 {
		return ErrBufferTooShort
	}
	bufferPathLen := int(binary.BigEndian.Uint16(data[offset : offset+2]))
	offset += 2

	// Read buffer path
	if len(data) < offset+bufferPathLen {
		return ErrBufferTooShort
	}
	f.bufferPath = string(data[offset : offset+bufferPathLen])

	f.initialized = true
	return nil
}

// Cleanup removes shared memory resources
func (f *FileBasedShmManager) Cleanup() error {
	var lastErr error
	if f.queuePath != "" {
		if err := os.Remove(f.queuePath); err != nil && !os.IsNotExist(err) {
			lastErr = err
		}
	}
	if f.bufferPath != "" {
		if err := os.Remove(f.bufferPath); err != nil && !os.IsNotExist(err) {
			lastErr = err
		}
	}
	f.initialized = false
	return lastErr
}

// GetQueuePath returns the queue shared memory path
func (f *FileBasedShmManager) GetQueuePath() string {
	return f.queuePath
}

// GetBufferPath returns the buffer shared memory path
func (f *FileBasedShmManager) GetBufferPath() string {
	return f.bufferPath
}

// GetType returns the memory mapping type
func (f *FileBasedShmManager) GetType() MemMapType {
	return MemMapTypeDevShmFile
}

// MemFDBasedShmManager implements SharedMemoryManager for version 3 (memfd-based)
type MemFDBasedShmManager struct {
	queueName  string
	bufferName string
	queueFd    int
	bufferFd   int
	initialized bool
}

const (
	memfdCreateName = "shmipc"
	memfdQueueSize  = 32 * 1024
	memfdBufferSize = 32 * 1024 * 1024
)

// NewMemFDBasedShmManager creates a new memfd-based shared memory manager
func NewMemFDBasedShmManager() *MemFDBasedShmManager {
	timestamp := strconv.Itoa(int(time.Now().UnixNano() % 1000000))

	return &MemFDBasedShmManager{
		queueName:  memfdCreateName + "_queue_" + timestamp,
		bufferName: memfdCreateName + "_buffer_" + timestamp,
		initialized: false,
	}
}

// memfdCreate creates an anonymous shared memory file using memfd_create syscall
func (m *MemFDBasedShmManager) memfdCreate(name string, size int) (int, error) {
	// Try to use memfd_create syscall first (Linux 3.17+)
	fd, err := unix.MemfdCreate(name, unix.MFD_CLOEXEC)
	if err == nil {
		// Set the size using ftruncate
		if err := unix.Ftruncate(fd, int64(size)); err != nil {
			unix.Close(fd)
			return -1, fmt.Errorf("ftruncate failed: %w", err)
		}
		return fd, nil
	}

	// Fallback to regular file creation if memfd_create is not available
	tmpFile := filepath.Join(os.TempDir(), name+"_shm")
	file, err := os.Create(tmpFile)
	if err != nil {
		return -1, fmt.Errorf("failed to create temporary file for memfd simulation: %w", err)
	}
	defer file.Close()

	// Set the size
	if err := file.Truncate(int64(size)); err != nil {
		os.Remove(tmpFile)
		return -1, fmt.Errorf("ftruncate failed: %w", err)
	}

	// Reopen to get file descriptor
	fd, err = syscall.Open(tmpFile, syscall.O_RDWR, 0600)
	if err != nil {
		os.Remove(tmpFile)
		return -1, fmt.Errorf("failed to open file: %w", err)
	}

	// Remove the file from filesystem (but keep fd open)
	os.Remove(tmpFile)

	return fd, nil
}

// Initialize sets up the memfd shared memory
func (m *MemFDBasedShmManager) Initialize() error {
	// Create queue memfd
	queueFd, err := m.memfdCreate(m.queueName, memfdQueueSize)
	if err != nil {
		return fmt.Errorf("failed to create queue memfd: %w", err)
	}

	// Create buffer memfd
	bufferFd, err := m.memfdCreate(m.bufferName, memfdBufferSize)
	if err != nil {
		syscall.Close(queueFd)
		return fmt.Errorf("failed to create buffer memfd: %w", err)
	}

	m.queueFd = queueFd
	m.bufferFd = bufferFd
	m.initialized = true
	return nil
}

// GenerateMetadata creates metadata payload for typeShareMemoryByMemfd
func (m *MemFDBasedShmManager) GenerateMetadata(version uint8, eventType eventType) ([]byte, error) {
	if !m.initialized {
		return nil, ErrShmNotInitialized
	}

	msg := &MessageShareMemoryByMemFD{
		Header: Header{
			Version: version,
		},
		QueueFile:  m.queueName,
		BufferFile: m.bufferName,
	}

	return msg.Append(nil), nil
}

// ParseMetadata processes received metadata from peer
func (m *MemFDBasedShmManager) ParseMetadata(data []byte) error {
	var msg MessageShareMemoryByMemFD
	if err := msg.Decode(data); err != nil {
		return err
	}

	m.queueName = msg.QueueFile
	m.bufferName = msg.BufferFile

	// Note: In a real implementation, we would receive the actual file descriptors
	// via Unix domain socket SCM_RIGHTS. For this example, we'll just store the names.
	m.initialized = true
	return nil
}

// Cleanup closes file descriptors
func (m *MemFDBasedShmManager) Cleanup() error {
	var lastErr error
	if m.queueFd >= 0 {
		if err := syscall.Close(m.queueFd); err != nil {
			lastErr = err
		}
		m.queueFd = -1
	}
	if m.bufferFd >= 0 {
		if err := syscall.Close(m.bufferFd); err != nil {
			lastErr = err
		}
		m.bufferFd = -1
	}
	m.initialized = false
	return lastErr
}

// GetQueuePath returns the queue shared memory name (for memfd)
func (m *MemFDBasedShmManager) GetQueuePath() string {
	return m.queueName
}

// GetBufferPath returns the buffer shared memory name (for memfd)
func (m *MemFDBasedShmManager) GetBufferPath() string {
	return m.bufferName
}

// GetType returns the memory mapping type
func (m *MemFDBasedShmManager) GetType() MemMapType {
	return MemMapTypeMemFd
}

// GetQueueFd returns the queue file descriptor
func (m *MemFDBasedShmManager) GetQueueFd() int {
	return m.queueFd
}

// GetBufferFd returns the buffer file descriptor
func (m *MemFDBasedShmManager) GetBufferFd() int {
	return m.bufferFd
}

// SendMetadataAndFDs sends MemFD metadata and file descriptors to peer via Unix domain socket
func (m *MemFDBasedShmManager) SendMetadataAndFDs(conn net.Conn, version uint8) error {
	// 1. Send metadata
	metadata, err := m.GenerateMetadata(version, typeShareMemoryByMemfd)
	if err != nil {
		return fmt.Errorf("failed to generate metadata: %w", err)
	}

	_, err = conn.Write(metadata)
	if err != nil {
		return fmt.Errorf("failed to send metadata: %w", err)
	}

	// 2. Wait for ack
	headerBuf := make([]byte, headerSize)
	_, err = conn.Read(headerBuf)
	if err != nil {
		return fmt.Errorf("failed to read ack: %w", err)
	}

	var header Header
	if err := header.Decode(headerBuf); err != nil {
		return fmt.Errorf("failed to decode ack header: %w", err)
	}

	if !header.IsValid() || header.Type != uint8(typeAckReadyRecvFD) {
		return fmt.Errorf("expected ack ready recv FD, got: magic=0x%x, type=%d", header.Magic, header.Type)
	}

	// 3. Send file descriptors
	err = sendFileDescriptors(conn, m.bufferFd, m.queueFd)
	if err != nil {
		return fmt.Errorf("failed to send file descriptors: %w", err)
	}

	return nil
}