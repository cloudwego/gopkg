package shmipc

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewQueue(t *testing.T) {
	queueCap := int64(10)
	mem := make([]byte, countQueueMemSize(queueCap))

	q := newQueue(mem, queueCap)
	assert.NotNil(t, q)
	assert.Equal(t, queueCap, q.cap)
	assert.True(t, q.IsEmpty())
	assert.False(t, q.IsFull())
}

func TestMappingQueue(t *testing.T) {
	queueCap := int64(10)
	mem := make([]byte, countQueueMemSize(queueCap))

	// Create original queue
	orig := newQueue(mem, queueCap)

	// Map from same memory
	mapped := mappingQueue(mem)
	assert.NotNil(t, mapped)
	assert.Equal(t, orig.cap, mapped.cap)
}

func TestQueue_PutPop(t *testing.T) {
	queueCap := int64(5)
	mem := make([]byte, countQueueMemSize(queueCap))
	q := newQueue(mem, queueCap)

	elem := QueueElement{
		StreamID: 123,
		Offset:   456,
		Status:   789,
	}

	// Put element
	err := q.Put(elem)
	require.NoError(t, err)
	assert.Equal(t, int64(1), q.Size())
	assert.False(t, q.IsEmpty())

	// Pop element
	popped, err := q.Pop()
	require.NoError(t, err)
	assert.Equal(t, elem.StreamID, popped.StreamID)
	assert.Equal(t, elem.Offset, popped.Offset)
	assert.Equal(t, elem.Status, popped.Status)
	assert.True(t, q.IsEmpty())
}

func TestQueue_PopEmpty(t *testing.T) {
	queueCap := int64(5)
	mem := make([]byte, countQueueMemSize(queueCap))
	q := newQueue(mem, queueCap)

	_, err := q.Pop()
	assert.ErrorIs(t, err, errQueueEmpty)
}

func TestQueue_PutFull(t *testing.T) {
	queueCap := int64(3)
	mem := make([]byte, countQueueMemSize(queueCap))
	q := newQueue(mem, queueCap)

	// Fill queue
	for i := 0; i < int(queueCap); i++ {
		err := q.Put(QueueElement{StreamID: uint32(i)})
		require.NoError(t, err)
	}

	assert.True(t, q.IsFull())

	// Try to add one more
	err := q.Put(QueueElement{StreamID: 999})
	assert.ErrorIs(t, err, ErrQueueFull)
}

func TestQueue_FIFO_Order(t *testing.T) {
	queueCap := int64(10)
	mem := make([]byte, countQueueMemSize(queueCap))
	q := newQueue(mem, queueCap)

	// Put multiple elements
	for i := 0; i < 5; i++ {
		err := q.Put(QueueElement{StreamID: uint32(i)})
		require.NoError(t, err)
	}

	// Pop and verify order
	for i := 0; i < 5; i++ {
		elem, err := q.Pop()
		require.NoError(t, err)
		assert.Equal(t, uint32(i), elem.StreamID)
	}
}

func TestQueue_WorkingFlag(t *testing.T) {
	queueCap := int64(5)
	mem := make([]byte, countQueueMemSize(queueCap))
	q := newQueue(mem, queueCap)

	// Initially not working
	assert.False(t, q.ConsumerIsWorking())

	// Mark as working
	assert.True(t, q.MarkWorking())
	assert.True(t, q.ConsumerIsWorking())

	// Try to mark again (should fail)
	assert.False(t, q.MarkWorking())

	// Mark not working
	q.MarkNotWorking()
	assert.False(t, q.ConsumerIsWorking())
}

func TestQueue_MarkNotWorking_WithData(t *testing.T) {
	queueCap := int64(5)
	mem := make([]byte, countQueueMemSize(queueCap))
	q := newQueue(mem, queueCap)

	// Add an element
	err := q.Put(QueueElement{StreamID: 1})
	require.NoError(t, err)

	// Mark not working - should set to 1 because queue not empty
	q.MarkNotWorking()
	assert.True(t, q.ConsumerIsWorking())
}

func TestQueue_Concurrent(t *testing.T) {
	queueCap := int64(50)
	mem := make([]byte, countQueueMemSize(queueCap))
	q := newQueue(mem, queueCap)

	numProducers := 2
	itemsPerProducer := 10
	totalItems := numProducers * itemsPerProducer

	var wg sync.WaitGroup
	consumedCount := atomic.Int64{}

	// Producers
	for p := 0; p < numProducers; p++ {
		wg.Add(1)
		go func(producerID int) {
			defer wg.Done()
			for i := 0; i < itemsPerProducer; i++ {
				streamID := uint32(producerID*1000 + i)
				for {
					err := q.Put(QueueElement{StreamID: streamID})
					if err == nil {
						break
					}
					// Retry if full (small sleep to avoid busy wait)
				}
			}
		}(p)
	}

	// Single consumer to consume all items
	wg.Add(1)
	go func() {
		defer wg.Done()
		for consumedCount.Load() < int64(totalItems) {
			_, err := q.Pop()
			if err == nil {
				consumedCount.Add(1)
			}
		}
	}()

	wg.Wait()

	// Verify all items were consumed
	assert.Equal(t, int64(totalItems), consumedCount.Load())
	assert.True(t, q.IsEmpty())
}

func TestNewQueueManagerFromMem(t *testing.T) {
	queueCap := int64(100)
	totalSize := countQueueMemSize(queueCap) * queueCount
	mem := make([]byte, totalSize)

	qm, err := newQueueManagerFromMem("test_path", mem, queueCap)
	require.NoError(t, err)
	assert.NotNil(t, qm)
	assert.Equal(t, "test_path", qm.GetPath())
	assert.NotNil(t, qm.GetSendQueue())
	assert.NotNil(t, qm.GetRecvQueue())
}

func TestNewQueueManagerFromMem_Insufficient(t *testing.T) {
	mem := make([]byte, 100) // Too small
	_, err := newQueueManagerFromMem("test", mem, 10)
	assert.Error(t, err)
}

func TestQueueManager_Cleanup(t *testing.T) {
	queueCap := int64(10)
	totalSize := countQueueMemSize(queueCap) * queueCount
	mem := make([]byte, totalSize)

	qm, err := newQueueManagerFromMem("test", mem, queueCap)
	require.NoError(t, err)

	err = qm.Cleanup()
	assert.NoError(t, err)
	assert.Nil(t, qm.sendQueue)
	assert.Nil(t, qm.recvQueue)
}

func TestQueueManager_GettersSetters(t *testing.T) {
	queueCap := int64(10)
	totalSize := countQueueMemSize(queueCap) * queueCount
	mem := make([]byte, totalSize)

	qm, err := newQueueManagerFromMem("test_path", mem, queueCap)
	require.NoError(t, err)

	assert.Equal(t, "test_path", qm.GetPath())
	assert.Equal(t, -1, qm.GetMemFd())
	assert.Equal(t, MemMapTypeDevShmFile, qm.GetType())
}

func TestCountQueueMemSize(t *testing.T) {
	tests := []struct {
		cap      int64
		expected int
	}{
		{0, queueHeaderLength},
		{1, queueHeaderLength + queueElementLen},
		{10, queueHeaderLength + 10*queueElementLen},
		{100, queueHeaderLength + 100*queueElementLen},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			size := countQueueMemSize(tt.cap)
			assert.Equal(t, tt.expected, size)
		})
	}
}
