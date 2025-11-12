package shmipc

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileBasedShmManager_Initialize(t *testing.T) {
	mgr := NewFileBasedShmManager()
	require.NotNil(t, mgr)

	err := mgr.Initialize()
	require.NoError(t, err)
	assert.True(t, mgr.initialized)

	// Verify files exist
	_, err = os.Stat(mgr.queuePath)
	assert.NoError(t, err)
	_, err = os.Stat(mgr.bufferPath)
	assert.NoError(t, err)

	// Cleanup
	err = mgr.Cleanup()
	assert.NoError(t, err)

	// Verify files removed
	_, err = os.Stat(mgr.queuePath)
	assert.True(t, os.IsNotExist(err))
}

func TestFileBasedShmManager_Getters(t *testing.T) {
	mgr := NewFileBasedShmManager()
	err := mgr.Initialize()
	require.NoError(t, err)
	defer mgr.Cleanup()

	assert.NotEmpty(t, mgr.GetQueuePath())
	assert.NotEmpty(t, mgr.GetBufferPath())
	assert.Equal(t, MemMapTypeDevShmFile, mgr.GetType())
	assert.NotNil(t, mgr.GetQueueManager())
	assert.NotNil(t, mgr.GetBufferManager())
}

func TestFileBasedShmManager_Managers(t *testing.T) {
	mgr := NewFileBasedShmManager()
	err := mgr.Initialize()
	require.NoError(t, err)
	defer mgr.Cleanup()

	// Test queue manager
	qm := mgr.GetQueueManager()
	require.NotNil(t, qm)
	assert.NotNil(t, qm.GetSendQueue())
	assert.NotNil(t, qm.GetRecvQueue())

	// Test buffer manager
	bm := mgr.GetBufferManager()
	require.NotNil(t, bm)

	// Allocate and recycle buffer
	buf, err := bm.AllocBuffer(128)
	require.NoError(t, err)
	assert.NotNil(t, buf)
	bm.RecycleBuffer(buf)
}

func TestMemFDBasedShmManager_Initialize(t *testing.T) {
	mgr := NewMemFDBasedShmManager()
	require.NotNil(t, mgr)

	err := mgr.Initialize()
	require.NoError(t, err)
	assert.True(t, mgr.initialized)
	assert.GreaterOrEqual(t, mgr.queueFd, 0)
	assert.GreaterOrEqual(t, mgr.bufferFd, 0)

	// Cleanup
	err = mgr.Cleanup()
	assert.NoError(t, err)
	assert.Equal(t, -1, mgr.queueFd)
	assert.Equal(t, -1, mgr.bufferFd)
}

func TestMemFDBasedShmManager_Getters(t *testing.T) {
	mgr := NewMemFDBasedShmManager()
	err := mgr.Initialize()
	require.NoError(t, err)
	defer mgr.Cleanup()

	assert.NotEmpty(t, mgr.GetQueuePath())
	assert.NotEmpty(t, mgr.GetBufferPath())
	assert.Equal(t, MemMapTypeMemFd, mgr.GetType())
	assert.NotNil(t, mgr.GetQueueManager())
	assert.NotNil(t, mgr.GetBufferManager())
	assert.GreaterOrEqual(t, mgr.GetQueueFd(), 0)
	assert.GreaterOrEqual(t, mgr.GetBufferFd(), 0)
}

func TestMemFDBasedShmManager_Managers(t *testing.T) {
	mgr := NewMemFDBasedShmManager()
	err := mgr.Initialize()
	require.NoError(t, err)
	defer mgr.Cleanup()

	// Test queue manager
	qm := mgr.GetQueueManager()
	require.NotNil(t, qm)

	elem := QueueElement{StreamID: 1, Offset: 100, Status: 0}
	err = qm.GetSendQueue().Put(elem)
	require.NoError(t, err)

	popped, err := qm.GetSendQueue().Pop()
	require.NoError(t, err)
	assert.Equal(t, elem.StreamID, popped.StreamID)

	// Test buffer manager
	bm := mgr.GetBufferManager()
	require.NotNil(t, bm)

	buf, err := bm.AllocBuffer(256)
	require.NoError(t, err)
	assert.NotNil(t, buf)
	bm.RecycleBuffer(buf)
}

func TestGenerateUniqueName(t *testing.T) {
	name1 := generateUniqueName()
	name2 := generateUniqueName()

	assert.NotEmpty(t, name1)
	assert.NotEmpty(t, name2)
	// Names should be different most of the time
	assert.NotEqual(t, name1, name2)
}

func TestGetDefaultBufferConfig(t *testing.T) {
	config := getDefaultBufferConfig()
	assert.NotEmpty(t, config)
	assert.Len(t, config, 3)

	// Verify total percentage is 100
	total := uint32(0)
	for _, pair := range config {
		total += pair.Percent
	}
	assert.Equal(t, uint32(100), total)
}

func TestRemoveIfExists(t *testing.T) {
	// Test with existing file
	tmpFile := filepath.Join(os.TempDir(), "test_remove_"+generateUniqueName())
	f, err := os.Create(tmpFile)
	require.NoError(t, err)
	f.Close()

	err = removeIfExists(tmpFile)
	assert.NoError(t, err)

	// Test with non-existing file
	err = removeIfExists("/nonexistent/file")
	assert.NoError(t, err)

	// Test with empty path
	err = removeIfExists("")
	assert.NoError(t, err)
}

func TestOpenAndMmapFile(t *testing.T) {
	tmpPath := filepath.Join(os.TempDir(), "test_mmap_"+generateUniqueName())
	defer os.Remove(tmpPath)

	mem, err := openAndMmapFile(tmpPath, 4096)
	require.NoError(t, err)
	assert.NotNil(t, mem)
	assert.Len(t, mem, 4096)
}

func TestCreateAndMmapMemfd(t *testing.T) {
	fd, mem, err := createAndMmapMemfd("test_memfd", 8192)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, fd, 0)
	assert.NotNil(t, mem)
	assert.Len(t, mem, 8192)

	// Cleanup
	closeFd(&fd)
}

func TestCreateManagers(t *testing.T) {
	queueMem := make([]byte, queueSize)
	bufferMem := make([]byte, bufferSize)

	qm, bm, err := createManagers("test_q", "test_b", queueMem, bufferMem)
	require.NoError(t, err)
	assert.NotNil(t, qm)
	assert.NotNil(t, bm)
}

func TestUnmapSharedMemory(t *testing.T) {
	// Create some mapped memory
	tmpPath1 := filepath.Join(os.TempDir(), "test_unmap1_"+generateUniqueName())
	tmpPath2 := filepath.Join(os.TempDir(), "test_unmap2_"+generateUniqueName())
	defer os.Remove(tmpPath1)
	defer os.Remove(tmpPath2)

	mem1, err := openAndMmapFile(tmpPath1, 4096)
	require.NoError(t, err)

	mem2, err := openAndMmapFile(tmpPath2, 4096)
	require.NoError(t, err)

	// Unmap both
	err = unmapSharedMemory(&mem1, &mem2)
	assert.NoError(t, err)
	assert.Nil(t, mem1)
	assert.Nil(t, mem2)
}

func TestCloseFd(t *testing.T) {
	// Create a file descriptor
	tmpPath := filepath.Join(os.TempDir(), "test_closefd_"+generateUniqueName())
	f, err := os.Create(tmpPath)
	require.NoError(t, err)
	defer os.Remove(tmpPath)

	fd := int(f.Fd())
	f.Close() // Close the file first

	// Try to close fd
	err = closeFd(&fd)
	// May succeed or fail depending on whether fd was already closed
	assert.Equal(t, -1, fd)
}

func TestFileBasedShmManager_Cleanup_Twice(t *testing.T) {
	mgr := NewFileBasedShmManager()
	err := mgr.Initialize()
	require.NoError(t, err)

	// First cleanup
	err = mgr.Cleanup()
	assert.NoError(t, err)

	// Second cleanup should not fail
	err = mgr.Cleanup()
	assert.NoError(t, err)
}

func TestMemFDBasedShmManager_Cleanup_Twice(t *testing.T) {
	mgr := NewMemFDBasedShmManager()
	err := mgr.Initialize()
	require.NoError(t, err)

	// First cleanup
	err = mgr.Cleanup()
	assert.NoError(t, err)

	// Second cleanup should not fail
	err = mgr.Cleanup()
	assert.NoError(t, err)
}
