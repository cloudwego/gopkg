// Copyright 2024 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bufiox

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBytesReader_BasicFunctionality(t *testing.T) {
	data := []byte("Hello, BytesReader!")
	reader := NewBytesReader(data)

	buf, err := reader.Next(5)
	require.NoError(t, err)
	assert.Equal(t, []byte("Hello"), buf)
	assert.Equal(t, 5, reader.ReadLen())

	peekBuf, err := reader.Peek(1)
	require.NoError(t, err)
	assert.Equal(t, []byte(","), peekBuf)
	assert.Equal(t, 5, reader.ReadLen())

	err = reader.Skip(1)
	require.NoError(t, err)
	assert.Equal(t, 6, reader.ReadLen())

	buf, err = reader.Next(6)
	require.NoError(t, err)
	assert.Equal(t, []byte(" Bytes"), buf)

	var binaryBuf [5]byte
	n, err := reader.ReadBinary(binaryBuf[:])
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, []byte("Reade"), binaryBuf[:])

	err = reader.Release(nil)
	require.NoError(t, err)
}

func TestBytesReader_BoundaryConditions(t *testing.T) {
	data := []byte("test")
	reader := NewBytesReader(data)

	t.Run("NegativeCount", func(t *testing.T) {
		_, err := reader.Next(-1)
		assert.Equal(t, errNegativeCount, err)

		_, err = reader.Peek(-1)
		assert.Equal(t, errNegativeCount, err)

		err = reader.Skip(-1)
		assert.Equal(t, errNegativeCount, err)
	})

	t.Run("ZeroCount", func(t *testing.T) {
		buf, err := reader.Next(0)
		require.NoError(t, err)
		assert.Equal(t, 0, len(buf))

		buf, err = reader.Peek(0)
		require.NoError(t, err)
		assert.Equal(t, 0, len(buf))

		err = reader.Skip(0)
		require.NoError(t, err)
	})

	t.Run("EmptySlice", func(t *testing.T) {
		emptyReader := NewBytesReader([]byte{})
		var emptyBuf []byte

		_, err := emptyReader.Next(1)
		assert.Equal(t, errNoRemainingData, err)

		_, err = emptyReader.Peek(1)
		assert.Equal(t, errNoRemainingData, err)

		err = emptyReader.Skip(1)
		assert.Equal(t, errNoRemainingData, err)

		n, err := emptyReader.ReadBinary(emptyBuf)
		require.NoError(t, err)
		assert.Equal(t, 0, n)
	})

	t.Run("ReadMoreThanAvailable", func(t *testing.T) {
		reader := NewBytesReader(data)

		_, err := reader.Next(10)
		assert.Equal(t, errNoRemainingData, err)

		_, err = reader.Peek(10)
		assert.Equal(t, errNoRemainingData, err)

		err = reader.Skip(10)
		assert.Equal(t, errNoRemainingData, err)
	})
}

// TestBytesReader_AdvancedFunctionality tests advanced BytesReader features
func TestBytesReader_AdvancedFunctionality(t *testing.T) {
	data := []byte("0123456789")
	reader := NewBytesReader(data)

	t.Run("PeekAfterNext", func(t *testing.T) {
		buf, err := reader.Next(3)
		require.NoError(t, err)
		assert.Equal(t, []byte("012"), buf)

		peekBuf, err := reader.Peek(3)
		require.NoError(t, err)
		assert.Equal(t, []byte("345"), peekBuf)

		assert.Equal(t, 3, reader.ReadLen())

		buf, err = reader.Next(3)
		require.NoError(t, err)
		assert.Equal(t, []byte("345"), buf)
		assert.Equal(t, 6, reader.ReadLen())
	})

	t.Run("SkipAfterPeek", func(t *testing.T) {
		peekBuf, err := reader.Peek(2)
		require.NoError(t, err)
		assert.Equal(t, []byte("67"), peekBuf)

		err = reader.Skip(2)
		require.NoError(t, err)
		assert.Equal(t, 8, reader.ReadLen())

		buf, err := reader.Next(2)
		require.NoError(t, err)
		assert.Equal(t, []byte("89"), buf)
		assert.Equal(t, 10, reader.ReadLen())
	})

	t.Run("PartialReadBinary", func(t *testing.T) {
		reader := NewBytesReader(data)

		var buf [5]byte
		n, err := reader.ReadBinary(buf[:3])
		require.NoError(t, err)
		assert.Equal(t, 3, n)
		assert.Equal(t, []byte("012"), buf[:3])

		n, err = reader.ReadBinary(buf[:])
		require.NoError(t, err)
		assert.Equal(t, 5, n)
		assert.Equal(t, []byte("34567"), buf[:])
	})
}

// TestBytesWriter_BasicFunctionality tests basic BytesWriter functionality
func TestBytesWriter_BasicFunctionality(t *testing.T) {
	var buf []byte
	writer := NewBytesWriter(&buf)

	mallocBuf, err := writer.Malloc(10)
	require.NoError(t, err)
	assert.Equal(t, 10, len(mallocBuf))
	copy(mallocBuf, []byte("0123456789"))
	assert.Equal(t, 10, writer.WrittenLen())

	n, err := writer.WriteBinary([]byte("Hello"))
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, 15, writer.WrittenLen())

	err = writer.Flush()
	require.NoError(t, err)
	assert.Equal(t, 0, writer.WrittenLen())
	assert.Equal(t, "0123456789Hello", string(buf))
}

// TestBytesWriter_BoundaryConditions tests boundary conditions for BytesWriter
func TestBytesWriter_BoundaryConditions(t *testing.T) {
	var buf []byte
	writer := NewBytesWriter(&buf)

	t.Run("NegativeCount", func(t *testing.T) {
		_, err := writer.Malloc(-1)
		assert.Equal(t, errNegativeCount, err)
	})

	t.Run("ZeroCount", func(t *testing.T) {
		mallocBuf, err := writer.Malloc(0)
		require.NoError(t, err)
		assert.Equal(t, 0, len(mallocBuf))
		assert.Equal(t, 0, writer.WrittenLen())
	})

	t.Run("EmptyWrite", func(t *testing.T) {
		var emptyBuf []byte
		writer := NewBytesWriter(&emptyBuf)

		n, err := writer.WriteBinary([]byte{})
		require.NoError(t, err)
		assert.Equal(t, 0, n)
		assert.Equal(t, 0, writer.WrittenLen())

		err = writer.Flush()
		require.NoError(t, err)
		assert.Equal(t, 0, len(emptyBuf))
	})

	t.Run("FlushWithoutData", func(t *testing.T) {
		var flushBuf []byte
		writer := NewBytesWriter(&flushBuf)

		err := writer.Flush()
		require.NoError(t, err)
		assert.Equal(t, 0, len(flushBuf))
	})
}

// TestBytesWriter_AdvancedFunctionality tests advanced BytesWriter features
func TestBytesWriter_AdvancedFunctionality(t *testing.T) {
	t.Run("BufferGrowth", func(t *testing.T) {
		var buf []byte
		writer := NewBytesWriter(&buf)

		// Write data that requires buffer growth
		largeData := make([]byte, 16*1024) // 16KB > defaultBufSize
		for i := range largeData {
			largeData[i] = byte(i % 256)
		}

		n, err := writer.WriteBinary(largeData)
		require.NoError(t, err)
		assert.Equal(t, len(largeData), n)
		assert.Equal(t, len(largeData), writer.WrittenLen())

		err = writer.Flush()
		require.NoError(t, err)
		assert.Equal(t, len(largeData), len(buf))
		assert.Equal(t, largeData, buf)
	})

	t.Run("MultipleMalloc", func(t *testing.T) {
		var buf []byte
		writer := NewBytesWriter(&buf)

		// Multiple small mallocs
		for i := 0; i < 10; i++ {
			mallocBuf, err := writer.Malloc(10)
			require.NoError(t, err)
			copy(mallocBuf, []byte("0123456789"))
		}

		assert.Equal(t, 100, writer.WrittenLen())

		err := writer.Flush()
		require.NoError(t, err)
		assert.Equal(t, 100, len(buf))
		assert.Equal(t, "0123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789", string(buf))
	})

	t.Run("MixedOperations", func(t *testing.T) {
		var buf []byte
		writer := NewBytesWriter(&buf)

		// Mix of Malloc and WriteBinary operations
		mallocBuf, err := writer.Malloc(5)
		require.NoError(t, err)
		copy(mallocBuf, []byte("Hello"))

		n, err := writer.WriteBinary([]byte("World"))
		require.NoError(t, err)
		assert.Equal(t, 5, n)

		mallocBuf, err = writer.Malloc(1)
		require.NoError(t, err)
		copy(mallocBuf, "!")

		assert.Equal(t, 11, writer.WrittenLen())

		err = writer.Flush()
		require.NoError(t, err)
		assert.Equal(t, "HelloWorld!", string(buf))
	})
}

// TestBytesReader_ReleaseBehavior tests Release behavior
func TestBytesReader_ReleaseBehavior(t *testing.T) {
	data := []byte("0123456789")
	reader := NewBytesReader(data)

	// Read some data
	buf, err := reader.Next(3)
	require.NoError(t, err)
	assert.Equal(t, []byte("012"), buf)
	assert.Equal(t, 3, reader.ReadLen())

	// Release and check behavior
	err = reader.Release(nil)
	require.NoError(t, err)
	assert.Equal(t, 0, reader.ReadLen())

	remainingBuf, err := reader.Next(7)
	require.NoError(t, err)
	assert.Equal(t, []byte("3456789"), remainingBuf)
	assert.Equal(t, 7, reader.ReadLen())

	err = reader.Release(nil)
	require.NoError(t, err)
	assert.Equal(t, 0, reader.ReadLen())
}

// TestBytesReaderAndWriter_Interaction tests interaction between BytesReader and BytesWriter
func TestBytesReaderAndWriter_Interaction(t *testing.T) {
	originalData := []byte("Hello, World!")

	// Write data using BytesWriter
	var buf []byte
	writer := NewBytesWriter(&buf)

	n, err := writer.WriteBinary(originalData)
	require.NoError(t, err)
	assert.Equal(t, len(originalData), n)

	err = writer.Flush()
	require.NoError(t, err)
	assert.Equal(t, originalData, buf)

	// Read data using BytesReader
	reader := NewBytesReader(buf)

	readData := make([]byte, len(originalData))
	n, err = reader.ReadBinary(readData)
	require.NoError(t, err)
	assert.Equal(t, len(originalData), n)
	assert.Equal(t, originalData, readData)

	assert.Equal(t, len(originalData), reader.ReadLen())

	_, err = reader.Next(1)
	assert.Equal(t, errNoRemainingData, err)

	err = reader.Release(nil)
	require.NoError(t, err)
	assert.Equal(t, 0, reader.ReadLen())
}

// TestBytesReader_ErrorConsistency tests that BytesReader returns consistent errors
func TestBytesReader_ErrorConsistency(t *testing.T) {
	data := []byte("test")
	reader := NewBytesReader(data)

	// Read all data
	buf, err := reader.Next(len(data))
	require.NoError(t, err)
	assert.Equal(t, data, buf)

	// All subsequent operations should return errNoRemainingData
	_, err = reader.Next(1)
	assert.Equal(t, errNoRemainingData, err)

	_, err = reader.Peek(1)
	assert.Equal(t, errNoRemainingData, err)

	err = reader.Skip(1)
	assert.Equal(t, errNoRemainingData, err)

	var readBuf [1]byte
	_, err = reader.ReadBinary(readBuf[:])
	assert.Equal(t, errNoRemainingData, err)
}

// TestBytesWriter_MultipleFlush tests multiple Flush operations
func TestBytesWriter_MultipleFlush(t *testing.T) {
	var buf []byte
	writer := NewBytesWriter(&buf)

	// Write some data
	_, err := writer.WriteBinary([]byte("Hello"))
	require.NoError(t, err)

	err = writer.Flush()
	require.NoError(t, err)
	assert.Equal(t, "Hello", string(buf))

	err = writer.Flush()
	require.NoError(t, err)
	assert.Equal(t, "Hello", string(buf))
}

// TestBytesWriter_AcquireSlowCoverage tests acquireSlow function branches
func TestBytesWriter_AcquireSlowCoverage(t *testing.T) {
	t.Run("InitialAllocation", func(t *testing.T) {
		var buf []byte
		writer := NewBytesWriter(&buf)

		mallocBuf, err := writer.Malloc(16 * 1024)
		require.NoError(t, err)
		assert.Equal(t, 16*1024, len(mallocBuf))

		_, err = writer.WriteBinary(make([]byte, 32*1024))
		require.NoError(t, err)

		err = writer.Flush()
		require.NoError(t, err)
		assert.Equal(t, 48*1024, len(buf))
	})

	t.Run("ExistingBufferGrowth", func(t *testing.T) {
		var buf []byte
		writer := NewBytesWriter(&buf)

		_, err := writer.WriteBinary([]byte("initial"))
		require.NoError(t, err)

		mallocBuf, err := writer.Malloc(16 * 1024)
		require.NoError(t, err)
		assert.Equal(t, 16*1024, len(mallocBuf))

		for i := 0; i < len(mallocBuf); i++ {
			mallocBuf[i] = byte(i % 256)
		}

		err = writer.Flush()
		require.NoError(t, err)
		assert.True(t, len(buf) > 16*1024)
	})
}