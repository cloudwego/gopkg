package shmipc

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
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

// generateUniqueName generates a unique name suffix using timestamp
func generateUniqueName() string {
	return strconv.Itoa(int(time.Now().UnixNano() % 1000000))
}

// NewFileBasedShmManager creates a new file-based shared memory manager
func NewFileBasedShmManager() *FileBasedShmManager {
	pid := os.Getpid()
	suffix := generateUniqueName()

	var baseDir string
	if runtime.GOOS == "linux" {
		baseDir = "/dev/shm"
	} else {
		baseDir = os.TempDir()
	}

	queueName := fmt.Sprintf("shmipc_queue_%d_%s", pid, suffix)
	bufferName := fmt.Sprintf("shmipc_buffer_%d_%s", pid, suffix)

	return &FileBasedShmManager{
		queuePath:  filepath.Join(baseDir, queueName),
		bufferPath: filepath.Join(baseDir, bufferName),
	}
}

const (
	queueSize  = 32 * 1024
	bufferSize = 32 * 1024 * 1024
)

// Initialize sets up the shared memory files
func (f *FileBasedShmManager) Initialize() error {
	if err := createShmFile(f.queuePath, queueSize); err != nil {
		return fmt.Errorf("failed to create queue shared memory: %w", err)
	}

	if err := createShmFile(f.bufferPath, bufferSize); err != nil {
		os.Remove(f.queuePath) // cleanup
		return fmt.Errorf("failed to create buffer shared memory: %w", err)
	}

	f.initialized = true
	return nil
}

// createShmFile creates a shared memory file with the given size
func createShmFile(path string, size int) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	return file.Truncate(int64(size))
}

// removeIfExists removes a file if it exists, ignoring not-exist errors
func removeIfExists(path string) error {
	if path == "" {
		return nil
	}
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// Cleanup removes shared memory resources
func (f *FileBasedShmManager) Cleanup() error {
	var lastErr error
	if err := removeIfExists(f.queuePath); err != nil {
		lastErr = err
	}
	if err := removeIfExists(f.bufferPath); err != nil {
		lastErr = err
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

const memfdCreateName = "shmipc"

// NewMemFDBasedShmManager creates a new memfd-based shared memory manager
func NewMemFDBasedShmManager() *MemFDBasedShmManager {
	suffix := generateUniqueName()

	return &MemFDBasedShmManager{
		queueName:  memfdCreateName + "_queue_" + suffix,
		bufferName: memfdCreateName + "_buffer_" + suffix,
		queueFd:    -1,
		bufferFd:   -1,
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
	queueFd, err := m.memfdCreate(m.queueName, queueSize)
	if err != nil {
		return fmt.Errorf("failed to create queue memfd: %w", err)
	}

	// Create buffer memfd
	bufferFd, err := m.memfdCreate(m.bufferName, bufferSize)
	if err != nil {
		syscall.Close(queueFd)
		return fmt.Errorf("failed to create buffer memfd: %w", err)
	}

	m.queueFd = queueFd
	m.bufferFd = bufferFd
	m.initialized = true
	return nil
}

// closeFd closes a file descriptor if valid (>= 0)
func closeFd(fd *int) error {
	if *fd >= 0 {
		err := syscall.Close(*fd)
		*fd = -1
		return err
	}
	return nil
}

// Cleanup closes file descriptors
func (m *MemFDBasedShmManager) Cleanup() error {
	var lastErr error
	if err := closeFd(&m.queueFd); err != nil {
		lastErr = err
	}
	if err := closeFd(&m.bufferFd); err != nil {
		lastErr = err
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
	if !m.initialized {
		return ErrShmNotInitialized
	}

	// 1. Send metadata
	msg := NewMessageShareMemory(version, m.queueName, m.bufferName)
	metadata := msg.AppendByType(nil, typeShareMemoryByMemfd)

	if _, err := conn.Write(metadata); err != nil {
		return fmt.Errorf("failed to send metadata: %w", err)
	}

	// 2. Wait for ack
	headerBuf := make([]byte, headerSize)
	if _, err := conn.Read(headerBuf); err != nil {
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
	if err := sendFileDescriptors(conn, m.bufferFd, m.queueFd); err != nil {
		return fmt.Errorf("failed to send file descriptors: %w", err)
	}

	return nil
}