package shmipc

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockConn implements net.Conn for testing
type mockConn struct {
	net.Conn
}

func (m *mockConn) Write(b []byte) (n int, err error) { return len(b), nil }
func (m *mockConn) Read(b []byte) (n int, err error)  { return 0, nil }
func (m *mockConn) Close() error                      { return nil }

// mockShmManager implements SharedMemoryManager for testing
type mockShmManager struct {
	bufferManager *BufferManager
	queueManager  *QueueManager
}

func (m *mockShmManager) Initialize() error                { return nil }
func (m *mockShmManager) Cleanup() error                   { return nil }
func (m *mockShmManager) GetQueuePath() string             { return "test" }
func (m *mockShmManager) GetBufferPath() string            { return "test" }
func (m *mockShmManager) GetType() MemMapType              { return MemMapTypeMemFd }
func (m *mockShmManager) GetQueueManager() *QueueManager   { return m.queueManager }
func (m *mockShmManager) GetBufferManager() *BufferManager { return m.bufferManager }

// setupTest creates a test environment with buffer and queue managers
func setupTest(t *testing.T, bufferSize, bufferMemSize, queueMemSize int) (*Client, *Stream) {
	// Create buffer manager
	pairs := []*SizePercentPair{{Size: uint32(bufferSize), Percent: 100}}
	bufferMem := make([]byte, bufferMemSize)
	bm, err := CreateBufferManager(pairs, "test", bufferMem)
	require.NoError(t, err)

	// Create queue manager
	queueCap := int64(1024)
	queueSize := countQueueMemSize(queueCap) * queueCount
	queueMem := make([]byte, queueSize)
	qm, err := newQueueManagerFromMem("test", queueMem, queueCap)
	require.NoError(t, err)

	// Create client with mock dependencies
	mockShmMgr := &mockShmManager{
		bufferManager: bm,
		queueManager:  qm,
	}
	client := &Client{
		conn:          &mockConn{},
		shmManager:    mockShmMgr,
		handshakeDone: true,
	}

	stream := newStream(client, 1)
	return client, stream
}

// TestStream_ReadWrite tests Stream.Read() and Stream.Write() with various data sizes
func TestStream_ReadWrite(t *testing.T) {
	tests := []struct {
		name       string
		dataSize   int
		bufferSize int
		bufferMem  int
	}{
		{"small_data", 13, 256, 64 * 1024},
		{"single_buffer", 256, 256, 64 * 1024},
		{"multi_buffer_1k", 1024, 256, 64 * 1024},
		{"multi_buffer_4k", 4 * 1024, 256, 128 * 1024},
		{"large_16k", 16 * 1024, 256, 512 * 1024},
		{"empty_data", 0, 256, 64 * 1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, stream := setupTest(t, tt.bufferSize, tt.bufferMem, 128*1024)

			// Create test data
			testData := make([]byte, tt.dataSize)
			for i := range testData {
				testData[i] = byte(i % 256)
			}

			// Write data
			n, err := stream.Write(testData)
			require.NoError(t, err)
			assert.Equal(t, len(testData), n)

			if tt.dataSize == 0 {
				// Empty data shouldn't enqueue anything
				return
			}

			// Get from send queue
			elem, err := client.shmManager.GetQueueManager().GetSendQueue().Pop()
			require.NoError(t, err)
			assert.Equal(t, uint32(1), elem.StreamID)

			// Deliver to receive channel
			stream.recvCh <- elem

			// Read data back
			readBuf := make([]byte, tt.dataSize)
			n, err = stream.Read(readBuf)
			require.NoError(t, err)
			assert.Equal(t, len(testData), n)
			assert.Equal(t, testData, readBuf[:n])
		})
	}
}

// TestStream_Read_PartialRead tests reading with smaller buffer than available data
func TestStream_Read_PartialRead(t *testing.T) {
	client, stream := setupTest(t, 256, 64*1024, 128*1024)

	// Create 400 bytes of test data (spans 2 buffers with 256-byte buffers)
	testData := make([]byte, 400)
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	// Write 400 bytes
	n, err := stream.Write(testData)
	require.NoError(t, err)
	assert.Equal(t, len(testData), n)

	// Get from send queue and deliver to receive channel
	elem, err := client.shmManager.GetQueueManager().GetSendQueue().Pop()
	require.NoError(t, err)
	stream.recvCh <- elem

	// Read 4 times with 100-byte buffer
	readData := make([]byte, 0, 400)
	readBuf := make([]byte, 100)

	for i := 0; i < 4; i++ {
		n, err := stream.Read(readBuf)
		require.NoError(t, err)
		assert.Equal(t, 100, n, "read %d should return 100 bytes", i+1)
		readData = append(readData, readBuf[:n]...)
	}

	// Verify all data was read correctly
	assert.Equal(t, testData, readData)
}
