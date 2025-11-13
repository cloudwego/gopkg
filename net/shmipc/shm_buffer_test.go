package shmipc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBufferHeader_Flags(t *testing.T) {
	headerBytes := make([]byte, bufferHeaderSize)
	header := BufferHeaderFromBytes(headerBytes)

	assert.False(t, header.hasNext())
	assert.False(t, header.isInUsed())

	header.setInUsed()
	assert.True(t, header.isInUsed())

	header.linkNext(100)
	assert.True(t, header.hasNext())
	assert.Equal(t, uint32(100), header.NextOff)

	header.clearFlag()
	assert.False(t, header.hasNext())
	assert.False(t, header.isInUsed())
}

func TestNewBuffer(t *testing.T) {
	headerBytes := make([]byte, bufferHeaderSize)
	header := BufferHeaderFromBytes(headerBytes)
	data := make([]byte, 1024)
	header.Cap = 1024

	// Test buffer creation
	bs := newBuffer(header, data, 0)
	assert.NotNil(t, bs)
	assert.Equal(t, uint32(1024), header.Cap)
	assert.Equal(t, uint32(0), header.Size)
}

func TestBuffer_ReadWrite(t *testing.T) {
	headerBytes := make([]byte, bufferHeaderSize)
	header := BufferHeaderFromBytes(headerBytes)
	data := make([]byte, 1024)
	header.Cap = 1024
	bs := newBuffer(header, data, 100)

	assert.Equal(t, 1024, len(bs.Buf()))

	bs.SetLen(10)
	assert.Equal(t, uint32(10), header.Size)
}

func TestBuffer_Reset(t *testing.T) {
	headerBytes := make([]byte, bufferHeaderSize)
	header := BufferHeaderFromBytes(headerBytes)
	data := make([]byte, 1024)
	header.Cap = 1024
	header.Size = 100
	header.DataStart = 50
	bs := newBuffer(header, data, 0)

	bs.reset()

	assert.Equal(t, uint32(0), header.Size)
	assert.Equal(t, uint32(0), header.DataStart)
}

func TestCreateBufferList(t *testing.T) {
	bufNum := uint32(10)
	capPerBuf := uint32(1024)
	memSize := countBufferListMemSize(bufNum, capPerBuf)
	mem := make([]byte, memSize)

	bl, err := createBufferList(bufNum, capPerBuf, mem, 0)
	require.NoError(t, err)
	assert.NotNil(t, bl)
	assert.Equal(t, int32(bufNum), bl.header.Size)
	assert.Equal(t, bufNum, bl.header.Cap)
	assert.Equal(t, capPerBuf, bl.header.CapPerBuf)
}

func TestCreateBufferList_Invalid(t *testing.T) {
	// Zero buffer number
	_, err := createBufferList(0, 1024, make([]byte, 1000), 0)
	assert.Error(t, err)

	// Zero capacity
	_, err = createBufferList(10, 0, make([]byte, 1000), 0)
	assert.Error(t, err)

	// Insufficient memory
	_, err = createBufferList(10, 1024, make([]byte, 100), 0)
	assert.Error(t, err)
}

func TestBufferList_PopPush(t *testing.T) {
	bufNum := uint32(5)
	capPerBuf := uint32(128)
	memSize := countBufferListMemSize(bufNum, capPerBuf)
	mem := make([]byte, memSize)

	bl, err := createBufferList(bufNum, capPerBuf, mem, 0)
	require.NoError(t, err)

	// Pop a buffer
	buf, err := bl.Pop()
	require.NoError(t, err)
	assert.NotNil(t, buf)
	assert.Equal(t, int(bufNum-2), bl.Remain()) // Size-1 for concurrent safety

	// Push it back
	bl.Push(buf)
	assert.Equal(t, int(bufNum-1), bl.Remain())
}

func TestBufferList_PopExhaust(t *testing.T) {
	bufNum := uint32(3)
	capPerBuf := uint32(64)
	memSize := countBufferListMemSize(bufNum, capPerBuf)
	mem := make([]byte, memSize)

	bl, err := createBufferList(bufNum, capPerBuf, mem, 0)
	require.NoError(t, err)

	// Pop all available buffers (one is reserved for concurrent safety)
	buf1, err := bl.Pop()
	require.NoError(t, err)
	assert.NotNil(t, buf1)

	buf2, err := bl.Pop()
	require.NoError(t, err)
	assert.NotNil(t, buf2)

	// Should fail now (1 buffer left is reserved)
	_, err = bl.Pop()
	assert.ErrorIs(t, err, ErrNoMoreBuffer)
}

func TestMappingBufferList(t *testing.T) {
	bufNum := uint32(3)
	capPerBuf := uint32(256)
	memSize := countBufferListMemSize(bufNum, capPerBuf)
	mem := make([]byte, memSize)

	// Create original list
	orig, err := createBufferList(bufNum, capPerBuf, mem, 0)
	require.NoError(t, err)

	// Map from same memory
	mapped, err := mappingBufferList(mem, 0)
	require.NoError(t, err)
	assert.Equal(t, orig.header.Cap, mapped.header.Cap)
	assert.Equal(t, orig.header.CapPerBuf, mapped.header.CapPerBuf)
}

func TestMappingBufferList_Invalid(t *testing.T) {
	// Too small buffer
	_, err := mappingBufferList(make([]byte, 10), 0)
	assert.Error(t, err)
}

func TestCreateBufferManager(t *testing.T) {
	pairs := []*SizePercentPair{
		{Size: 128, Percent: 50},
		{Size: 256, Percent: 50},
	}

	memSize := 64 * 1024 // 64KB
	mem := make([]byte, memSize)

	bm, err := CreateBufferManager(pairs, "test_path", mem)
	require.NoError(t, err)
	assert.NotNil(t, bm)
	assert.Equal(t, "test_path", bm.GetPath())
	assert.Len(t, bm.lists, 2)
}

func TestCreateBufferManager_InvalidPercent(t *testing.T) {
	pairs := []*SizePercentPair{
		{Size: 128, Percent: 60},
		{Size: 256, Percent: 50},
	}

	mem := make([]byte, 64*1024)
	_, err := CreateBufferManager(pairs, "test", mem)
	assert.Error(t, err)
}

func TestBufferManager_AllocRecycle(t *testing.T) {
	pairs := []*SizePercentPair{
		{Size: 128, Percent: 100},
	}

	mem := make([]byte, 64*1024)
	bm, err := CreateBufferManager(pairs, "test", mem)
	require.NoError(t, err)

	// Allocate buffer
	buf, err := bm.AllocBuffer(100)
	require.NoError(t, err)
	assert.NotNil(t, buf)

	// Recycle it
	bm.RecycleBuffer(buf)
}

func TestBufferManager_AllocTooLarge(t *testing.T) {
	pairs := []*SizePercentPair{
		{Size: 128, Percent: 100},
	}

	mem := make([]byte, 64*1024)
	bm, err := CreateBufferManager(pairs, "test", mem)
	require.NoError(t, err)

	// Try to allocate larger than max
	_, err = bm.AllocBuffer(10000)
	assert.ErrorIs(t, err, ErrNoMoreBuffer)
}

func TestBufferManager_TryAllocBuffer(t *testing.T) {
	pairs := []*SizePercentPair{
		{Size: 128, Percent: 50},
		{Size: 256, Percent: 50},
	}

	mem := make([]byte, 64*1024)
	bm, err := CreateBufferManager(pairs, "test", mem)
	require.NoError(t, err)

	// Try to allocate smaller size - should use normal allocation
	buf, err := bm.TryAllocBuffer(100)
	require.NoError(t, err)
	assert.NotNil(t, buf)
	assert.Equal(t, uint32(128), buf.header.Cap)
	bm.RecycleBuffer(buf)

	// Try to allocate size >= maxSliceSize - should allocate max-size buffer
	buf, err = bm.TryAllocBuffer(300)
	require.NoError(t, err)
	assert.NotNil(t, buf)
	assert.Equal(t, uint32(256), buf.header.Cap) // Should get max-size buffer
	bm.RecycleBuffer(buf)

	// Try to allocate exactly maxSliceSize - should allocate max-size buffer
	buf, err = bm.TryAllocBuffer(256)
	require.NoError(t, err)
	assert.NotNil(t, buf)
	assert.Equal(t, uint32(256), buf.header.Cap)
	bm.RecycleBuffer(buf)
}

func TestMappingBufferManager(t *testing.T) {
	pairs := []*SizePercentPair{
		{Size: 128, Percent: 100},
	}

	mem := make([]byte, 64*1024)
	orig, err := CreateBufferManager(pairs, "test", mem)
	require.NoError(t, err)

	// Map from same memory
	mapped, err := MappingBufferManager("test", mem)
	require.NoError(t, err)
	assert.Equal(t, orig.GetPath(), mapped.GetPath())
	assert.Len(t, mapped.lists, len(orig.lists))
}

func TestValidateSizePercentPairs(t *testing.T) {
	tests := []struct {
		name    string
		pairs   []*SizePercentPair
		wantErr bool
	}{
		{
			name:    "valid sizes",
			pairs:   []*SizePercentPair{{Size: 128, Percent: 100}},
			wantErr: false,
		},
		{
			name:    "empty pairs",
			pairs:   []*SizePercentPair{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSizePercentPairs(tt.pairs)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBufferManager_ReadBuffer(t *testing.T) {
	pairs := []*SizePercentPair{{Size: 128, Percent: 100}}
	mem := make([]byte, 64*1024)
	bm, err := CreateBufferManager(pairs, "test", mem)
	require.NoError(t, err)

	// Allocate a buffer
	buf, err := bm.AllocBuffer(100)
	require.NoError(t, err)

	// Read it back using offset
	readBuf, err := bm.ReadBuffer(buf.Offset())
	require.NoError(t, err)
	assert.Equal(t, buf.Offset(), readBuf.Offset())
}

func TestBufferManager_ReadBuffer_Invalid(t *testing.T) {
	pairs := []*SizePercentPair{{Size: 128, Percent: 100}}
	mem := make([]byte, 1024)
	bm, err := CreateBufferManager(pairs, "test", mem)
	require.NoError(t, err)

	// Try to read beyond bounds
	_, err = bm.ReadBuffer(99999)
	assert.Error(t, err)
}

func TestBufferChain(t *testing.T) {
	pairs := []*SizePercentPair{{Size: 64, Percent: 100}}
	mem := make([]byte, 16*1024)
	bm, err := CreateBufferManager(pairs, "test", mem)
	require.NoError(t, err)

	// Test allocBufferChain - single buffer
	data := []byte("hello")
	head, err := allocBufferChain(bm, data)
	require.NoError(t, err)
	assert.Equal(t, data, head.Data())
	assert.Nil(t, head.next)

	// Test bufferChainSize and bufferChainBytes
	assert.Equal(t, int64(5), bufferChainSize(head))
	assert.Equal(t, data, bufferChainBytes(head))

	// Test recycleBufferChain
	recycleBufferChain(bm, head)

	// Test allocBufferChain - multiple buffers
	largeData := make([]byte, 200)
	for i := range largeData {
		largeData[i] = byte(i)
	}
	head, err = allocBufferChain(bm, largeData)
	require.NoError(t, err)
	assert.NotNil(t, head.next)
	assert.Equal(t, int64(200), bufferChainSize(head))
	assert.Equal(t, largeData, bufferChainBytes(head))
	recycleBufferChain(bm, head)

	// Test nil cases
	assert.Equal(t, int64(0), bufferChainSize(nil))
	assert.Empty(t, bufferChainBytes(nil))
	recycleBufferChain(bm, nil) // Should not panic
}
