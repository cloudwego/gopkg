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

var shmBaseDir string

func init() {
	if runtime.GOOS == "linux" {
		shmBaseDir = "/dev/shm"
	} else {
		shmBaseDir = os.TempDir()
	}
}

// MemMapType specifies the shared memory mapping type
type MemMapType uint8

const (
	// MemMapTypeDevShmFile uses file-based tmpfs (/dev/shm on Linux, os.TempDir() on others)
	MemMapTypeDevShmFile MemMapType = 0
	// MemMapTypeMemFd uses memfd (Linux v3.17+)
	MemMapTypeMemFd MemMapType = 1
)

// SharedMemoryManager manages shared memory regions for queue and buffer
type SharedMemoryManager interface {
	Initialize() error
	Cleanup() error
	GetQueuePath() string
	GetBufferPath() string
	GetType() MemMapType
	GetQueueManager() *QueueManager
	GetBufferManager() *BufferManager
}

// FileBasedShmManager implements SharedMemoryManager using file-based shared memory
type FileBasedShmManager struct {
	queuePath     string
	bufferPath    string
	queueManager  *QueueManager
	bufferManager *BufferManager
	queueMem      []byte
	bufferMem     []byte
	initialized   bool
}

func generateUniqueName() string {
	return strconv.Itoa(int(time.Now().UnixNano() % 1000000))
}

// NewFileBasedShmManager creates a file-based shared memory manager
func NewFileBasedShmManager() *FileBasedShmManager {
	pid := os.Getpid()
	suffix := generateUniqueName()

	queueName := fmt.Sprintf("shmipc_queue_%d_%s", pid, suffix)
	bufferName := fmt.Sprintf("shmipc_buffer_%d_%s", pid, suffix)

	return &FileBasedShmManager{
		queuePath:  filepath.Join(shmBaseDir, queueName),
		bufferPath: filepath.Join(shmBaseDir, bufferName),
	}
}

const (
	queueSize  = 32 * 1024
	bufferSize = 32 * 1024 * 1024
	queueCap   = 1024
)

func getDefaultBufferConfig() []*SizePercentPair {
	return []*SizePercentPair{
		{Size: 1024, Percent: 20},
		{Size: 8192, Percent: 40},
		{Size: 65536, Percent: 40},
	}
}

func createManagers(queuePath, bufferPath string, queueMem, bufferMem []byte) (*QueueManager, *BufferManager, error) {
	queueMgr, err := newQueueManagerFromMem(queuePath, queueMem, queueCap)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create queue manager: %w", err)
	}

	bufferMgr, err := CreateBufferManager(getDefaultBufferConfig(), bufferPath, bufferMem)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create buffer manager: %w", err)
	}

	return queueMgr, bufferMgr, nil
}

func unmapSharedMemory(queueMem, bufferMem *[]byte) error {
	var lastErr error

	if *queueMem != nil {
		if err := unix.Munmap(*queueMem); err != nil {
			lastErr = err
		}
		*queueMem = nil
	}

	if *bufferMem != nil {
		if err := unix.Munmap(*bufferMem); err != nil {
			lastErr = err
		}
		*bufferMem = nil
	}

	return lastErr
}

func openAndMmapFile(path string, size int) ([]byte, error) {
	fd, err := syscall.Open(path, syscall.O_RDWR|syscall.O_CREAT|syscall.O_TRUNC, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}
	defer syscall.Close(fd)

	if err := syscall.Ftruncate(fd, int64(size)); err != nil {
		os.Remove(path)
		return nil, fmt.Errorf("failed to resize file: %w", err)
	}

	mem, err := unix.Mmap(fd, 0, size, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		os.Remove(path)
		return nil, fmt.Errorf("failed to mmap file: %w", err)
	}

	return mem, nil
}

// Initialize creates and mmaps queue and buffer files
func (f *FileBasedShmManager) Initialize() error {
	queueMem, err := openAndMmapFile(f.queuePath, queueSize)
	if err != nil {
		return fmt.Errorf("failed to open queue file: %w", err)
	}
	f.queueMem = queueMem

	bufferMem, err := openAndMmapFile(f.bufferPath, bufferSize)
	if err != nil {
		unix.Munmap(f.queueMem)
		os.Remove(f.queuePath)
		return fmt.Errorf("failed to open buffer file: %w", err)
	}
	f.bufferMem = bufferMem

	queueMgr, bufferMgr, err := createManagers(f.queuePath, f.bufferPath, queueMem, bufferMem)
	if err != nil {
		unmapSharedMemory(&f.queueMem, &f.bufferMem)
		os.Remove(f.queuePath)
		os.Remove(f.bufferPath)
		return err
	}
	f.queueManager = queueMgr
	f.bufferManager = bufferMgr

	f.initialized = true
	return nil
}

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

// Cleanup unmaps memory and removes shared memory files
func (f *FileBasedShmManager) Cleanup() error {
	lastErr := unmapSharedMemory(&f.queueMem, &f.bufferMem)

	if err := removeIfExists(f.queuePath); err != nil {
		lastErr = err
	}
	if err := removeIfExists(f.bufferPath); err != nil {
		lastErr = err
	}

	f.queueManager = nil
	f.bufferManager = nil
	f.initialized = false
	return lastErr
}

// GetQueuePath returns the queue file path
func (f *FileBasedShmManager) GetQueuePath() string {
	return f.queuePath
}

// GetBufferPath returns the buffer file path
func (f *FileBasedShmManager) GetBufferPath() string {
	return f.bufferPath
}

// GetType returns MemMapTypeDevShmFile
func (f *FileBasedShmManager) GetType() MemMapType {
	return MemMapTypeDevShmFile
}

// GetQueueManager returns the queue manager
func (f *FileBasedShmManager) GetQueueManager() *QueueManager {
	return f.queueManager
}

// GetBufferManager returns the buffer manager
func (f *FileBasedShmManager) GetBufferManager() *BufferManager {
	return f.bufferManager
}

// MemFDBasedShmManager implements SharedMemoryManager using memfd-based shared memory
type MemFDBasedShmManager struct {
	queueName     string
	bufferName    string
	queueFd       int
	bufferFd      int
	queueManager  *QueueManager
	bufferManager *BufferManager
	queueMem      []byte
	bufferMem     []byte
	initialized   bool
}

const memfdCreateName = "shmipc"

// NewMemFDBasedShmManager creates a memfd-based shared memory manager
func NewMemFDBasedShmManager() *MemFDBasedShmManager {
	suffix := generateUniqueName()

	return &MemFDBasedShmManager{
		queueName:  memfdCreateName + "_queue_" + suffix,
		bufferName: memfdCreateName + "_buffer_" + suffix,
		queueFd:    -1,
		bufferFd:   -1,
	}
}

func createAndMmapMemfd(name string, size int) (fd int, mem []byte, err error) {
	fd, err = unix.MemfdCreate(name, unix.MFD_CLOEXEC)
	if err == nil {
		if err := unix.Ftruncate(fd, int64(size)); err != nil {
			unix.Close(fd)
			return -1, nil, fmt.Errorf("ftruncate failed: %w", err)
		}
	} else {
		// Fallback if memfd_create unavailable
		tmpFile := filepath.Join(os.TempDir(), name+"_shm")
		file, err := os.Create(tmpFile)
		if err != nil {
			return -1, nil, fmt.Errorf("failed to create temporary file for memfd simulation: %w", err)
		}
		file.Close()

		if err := os.Truncate(tmpFile, int64(size)); err != nil {
			os.Remove(tmpFile)
			return -1, nil, fmt.Errorf("ftruncate failed: %w", err)
		}

		fd, err = syscall.Open(tmpFile, syscall.O_RDWR, 0600)
		if err != nil {
			os.Remove(tmpFile)
			return -1, nil, fmt.Errorf("failed to open file: %w", err)
		}

		os.Remove(tmpFile)
	}

	mem, err = unix.Mmap(fd, 0, size, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		syscall.Close(fd)
		return -1, nil, fmt.Errorf("failed to mmap: %w", err)
	}

	return fd, mem, nil
}

// Initialize creates and mmaps queue and buffer memfds
func (m *MemFDBasedShmManager) Initialize() error {
	queueFd, queueMem, err := createAndMmapMemfd(m.queueName, queueSize)
	if err != nil {
		return fmt.Errorf("failed to create queue memfd: %w", err)
	}
	m.queueFd = queueFd
	m.queueMem = queueMem

	bufferFd, bufferMem, err := createAndMmapMemfd(m.bufferName, bufferSize)
	if err != nil {
		unix.Munmap(m.queueMem)
		syscall.Close(m.queueFd)
		return fmt.Errorf("failed to create buffer memfd: %w", err)
	}
	m.bufferFd = bufferFd
	m.bufferMem = bufferMem

	queueMgr, bufferMgr, err := createManagers(m.queueName, m.bufferName, queueMem, bufferMem)
	if err != nil {
		unmapSharedMemory(&m.queueMem, &m.bufferMem)
		syscall.Close(m.queueFd)
		syscall.Close(m.bufferFd)
		return err
	}
	m.queueManager = queueMgr
	m.bufferManager = bufferMgr

	m.initialized = true
	return nil
}

func closeFd(fd *int) error {
	if *fd >= 0 {
		err := syscall.Close(*fd)
		*fd = -1
		return err
	}
	return nil
}

// Cleanup unmaps memory and closes file descriptors
func (m *MemFDBasedShmManager) Cleanup() error {
	lastErr := unmapSharedMemory(&m.queueMem, &m.bufferMem)

	if err := closeFd(&m.queueFd); err != nil {
		lastErr = err
	}
	if err := closeFd(&m.bufferFd); err != nil {
		lastErr = err
	}

	m.queueManager = nil
	m.bufferManager = nil
	m.initialized = false
	return lastErr
}

// GetQueuePath returns the queue memfd name
func (m *MemFDBasedShmManager) GetQueuePath() string {
	return m.queueName
}

// GetBufferPath returns the buffer memfd name
func (m *MemFDBasedShmManager) GetBufferPath() string {
	return m.bufferName
}

// GetType returns MemMapTypeMemFd
func (m *MemFDBasedShmManager) GetType() MemMapType {
	return MemMapTypeMemFd
}

// GetQueueManager returns the queue manager
func (m *MemFDBasedShmManager) GetQueueManager() *QueueManager {
	return m.queueManager
}

// GetBufferManager returns the buffer manager
func (m *MemFDBasedShmManager) GetBufferManager() *BufferManager {
	return m.bufferManager
}

// GetQueueFd returns the queue file descriptor
func (m *MemFDBasedShmManager) GetQueueFd() int {
	return m.queueFd
}

// GetBufferFd returns the buffer file descriptor
func (m *MemFDBasedShmManager) GetBufferFd() int {
	return m.bufferFd
}

// SendMetadataAndFDs sends memfd metadata and file descriptors via Unix domain socket
func (m *MemFDBasedShmManager) SendMetadataAndFDs(conn net.Conn, version uint8) error {
	if !m.initialized {
		return ErrShmNotInitialized
	}

	msg := NewMessageShareMemory(version, typeShareMemoryByMemfd, m.queueName, m.bufferName)
	metadata := msg.Append(nil)

	if _, err := conn.Write(metadata); err != nil {
		return fmt.Errorf("failed to send metadata: %w", err)
	}

	headerBuf := make([]byte, headerSize)
	if _, err := conn.Read(headerBuf); err != nil {
		return fmt.Errorf("failed to read ack: %w", err)
	}

	var header Header
	if err := header.Decode(headerBuf); err != nil {
		return fmt.Errorf("failed to decode ack header: %w", err)
	}

	if !header.IsValid() {
		return ErrInvalid
	}

	if err := sendFileDescriptors(conn, m.bufferFd, m.queueFd); err != nil {
		return fmt.Errorf("failed to send file descriptors: %w", err)
	}

	return nil
}
