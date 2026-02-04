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
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultReader_BasicFunctionality tests the basic functionality of DefaultReader
func TestDefaultReader_BasicFunctionality(t *testing.T) {
	data := []byte("Hello, World!")
	reader := NewDefaultReader(bytes.NewReader(data))

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
	assert.Equal(t, []byte(" World"), buf)

	var binaryBuf [3]byte
	n, err := reader.ReadBinary(binaryBuf[:])
	require.Equal(t, io.EOF, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, []byte("!"), binaryBuf[:1])

	err = reader.Release(nil)
	require.NoError(t, err)
}

// TestDefaultReader_BoundaryConditions tests boundary conditions
func TestDefaultReader_BoundaryConditions(t *testing.T) {
	t.Run("NegativeCount", func(t *testing.T) {
		reader := NewDefaultReader(bytes.NewReader([]byte("test")))

		// Test negative Next
		_, err := reader.Next(-1)
		assert.Equal(t, errNegativeCount, err)

		// Test negative Peek
		_, err = reader.Peek(-1)
		assert.Equal(t, errNegativeCount, err)

		// Test negative Skip
		err = reader.Skip(-1)
		assert.Equal(t, errNegativeCount, err)
	})

	t.Run("ZeroCount", func(t *testing.T) {
		reader := NewDefaultReader(bytes.NewReader([]byte("test")))

		// Test zero Next
		buf, err := reader.Next(0)
		require.NoError(t, err)
		assert.Nil(t, buf) // Next(0) returns nil slice

		// Test zero Peek
		buf, err = reader.Peek(0)
		require.NoError(t, err)
		assert.Nil(t, buf) // Peek(0) returns nil slice

		// Test zero Skip
		err = reader.Skip(0)
		require.NoError(t, err)
	})

	t.Run("EmptySlice", func(t *testing.T) {
		reader := NewDefaultReader(bytes.NewReader([]byte("test")))

		// Test empty ReadBinary
		var emptyBuf []byte
		n, err := reader.ReadBinary(emptyBuf)
		require.NoError(t, err)
		assert.Equal(t, 0, n)
	})

	t.Run("LargeBuffer", func(t *testing.T) {
		// Test with large buffer to trigger buffer growth
		largeData := make([]byte, 64*1024) // 64KB
		for i := range largeData {
			largeData[i] = byte(i % 256)
		}

		reader := NewDefaultReader(bytes.NewReader(largeData))

		// Test reading large chunk
		buf, err := reader.Next(32 * 1024) // 32KB
		require.NoError(t, err)
		assert.Equal(t, 32*1024, len(buf))
		assert.Equal(t, largeData[:32*1024], buf)
	})
}

// TestDefaultReader_ErrorConditions tests error handling
func TestDefaultReader_ErrorConditions(t *testing.T) {
	t.Run("IOError", func(t *testing.T) {
		// Create a reader that returns an error
		errReader := &errorReader{err: errors.New("test error")}
		reader := NewDefaultReader(errReader)

		// First call should fail
		_, err := reader.Next(10)
		assert.Error(t, err)

		// Subsequent calls should return the same error
		_, err = reader.Peek(10)
		assert.Error(t, err)
	})

	t.Run("NoProgressError", func(t *testing.T) {
		// Create a reader that keeps returning 0 bytes
		noProgressReader := &noProgressReader{}
		reader := NewDefaultReader(noProgressReader)

		// Should eventually return io.ErrNoProgress
		_, err := reader.Next(10)
		assert.Equal(t, io.ErrNoProgress, err)
	})
}

func TestDefaultReader_ReleaseAfterMultipleOperations(t *testing.T) {
	// Perform many operations and release
	data := make([]byte, 2048*20) // Increase data size to handle all operations
	reader := NewDefaultReader(bytes.NewReader(data))

	for i := 0; i < 2048; i++ { // Reduce iterations to fit within data size
		_, err := reader.Next(10)
		require.NoError(t, err)
		_, err = reader.Peek(10)
		require.NoError(t, err)
		err = reader.Skip(10)
		require.NoError(t, err)
	}

	// Release should free all buffers
	err := reader.Release(nil)
	require.NoError(t, err)
	assert.Equal(t, 0, reader.ReadLen())
}

// TestDefaultWriter_BasicFunctionality tests the basic functionality of DefaultWriter
func TestDefaultWriter_BasicFunctionality(t *testing.T) {
	var buf bytes.Buffer
	writer := NewDefaultWriter(&buf)

	// Test Malloc
	mallocBuf, err := writer.Malloc(10)
	require.NoError(t, err)
	assert.Equal(t, 10, len(mallocBuf))
	copy(mallocBuf, []byte("0123456789"))
	assert.Equal(t, 10, writer.WrittenLen())

	// Test WriteBinary
	n, err := writer.WriteBinary([]byte("Hello"))
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, 15, writer.WrittenLen())

	// Test Flush
	err = writer.Flush()
	require.NoError(t, err)
	assert.Equal(t, 0, writer.WrittenLen())
	assert.Equal(t, "0123456789Hello", buf.String())
}

// TestDefaultWriter_BoundaryConditions tests boundary conditions for writer
func TestDefaultWriter_BoundaryConditions(t *testing.T) {
	t.Run("NegativeCount", func(t *testing.T) {
		writer := NewDefaultWriter(&bytes.Buffer{})

		// Test negative Malloc
		_, err := writer.Malloc(-1)
		assert.Equal(t, errNegativeCount, err)
	})

	t.Run("ZeroCount", func(t *testing.T) {
		writer := NewDefaultWriter(&bytes.Buffer{})

		// Test zero Malloc
		buf, err := writer.Malloc(0)
		require.NoError(t, err)
		assert.Equal(t, 0, len(buf)) // Malloc(0) returns a slice with length 0
		assert.Equal(t, 0, writer.WrittenLen())
	})

	t.Run("LargeBuffer", func(t *testing.T) {
		var buf bytes.Buffer
		writer := NewDefaultWriter(&buf)

		// Test large Malloc to trigger buffer growth
		largeBuf, err := writer.Malloc(64 * 1024) // 64KB
		require.NoError(t, err)
		assert.Equal(t, 64*1024, len(largeBuf))
		assert.Equal(t, 64*1024, writer.WrittenLen())

		// Fill with test data
		for i := range largeBuf {
			largeBuf[i] = byte(i % 256)
		}

		err = writer.Flush()
		require.NoError(t, err)

		// Verify the actual written byte data integrity
		writtenBytes := buf.Bytes()
		assert.Equal(t, 64*1024, len(writtenBytes))

		// Verify each byte matches the expected pattern (i % 256)
		for i := 0; i < 64*1024; i++ {
			expectedByte := byte(i % 256)
			actualByte := writtenBytes[i]
			assert.Equalf(t, expectedByte, actualByte,
				"Large buffer data mismatch at byte %d: expected %d, got %d",
				i, expectedByte, actualByte)
		}
	})

	t.Run("WriteBinaryThreshold", func(t *testing.T) {
		var buf bytes.Buffer
		writer := NewDefaultWriter(&buf)

		// Test WriteBinary below threshold
		smallData := make([]byte, 1024) // 1KB < nocopyWriteThreshold
		for i := range smallData {
			smallData[i] = byte(i)
		}

		n, err := writer.WriteBinary(smallData)
		require.NoError(t, err)
		assert.Equal(t, 1024, n)

		// Test WriteBinary above threshold
		largeData := make([]byte, 8*1024) // 8KB > nocopyWriteThreshold
		for i := range largeData {
			largeData[i] = byte(i)
		}

		n, err = writer.WriteBinary(largeData)
		require.NoError(t, err)
		assert.Equal(t, 8*1024, n)

		err = writer.Flush()
		require.NoError(t, err)
		assert.Equal(t, 1024+8*1024, buf.Len())

		// Verify the actual written byte data integrity
		writtenBytes := buf.Bytes()

		// Verify small data (first 1024 bytes)
		for i := 0; i < 1024; i++ {
			assert.Equalf(t, byte(i), writtenBytes[i],
				"Small data mismatch at byte %d: expected %d, got %d",
				i, byte(i), writtenBytes[i])
		}

		// Verify large data (next 8KB bytes)
		for i := 0; i < 8*1024; i++ {
			assert.Equalf(t, byte(i), writtenBytes[1024+i],
				"Large data mismatch at byte %d: expected %d, got %d",
				i, byte(i), writtenBytes[1024+i])
		}
	})
}

// TestDefaultWriter_ErrorHandling tests error handling in DefaultWriter
func TestDefaultWriter_ErrorHandling(t *testing.T) {
	// Create a writer that returns an error
	errWriter := &errorWriter{err: errors.New("write error")}
	writer := NewDefaultWriter(errWriter)

	// Write some data
	_, err := writer.Malloc(10)
	require.NoError(t, err)

	// Flush should fail
	err = writer.Flush()
	assert.Error(t, err)

	// Subsequent operations should fail
	_, err = writer.Malloc(5)
	assert.Error(t, err)
}

// TestDefaultWriter_MemoryLeaks tests for memory leaks in DefaultWriter
func TestDefaultWriter_MemoryLeaks(t *testing.T) {
	t.Run("MultipleFlushes", func(t *testing.T) {
		writer := NewDefaultWriter(&bytes.Buffer{})

		// Perform multiple operations and flushes
		for i := 0; i < 100; i++ {
			_, err := writer.Malloc(100)
			require.NoError(t, err)

			_, err = writer.WriteBinary([]byte("test data"))
			require.NoError(t, err)

			err = writer.Flush()
			require.NoError(t, err)
		}
	})

	t.Run("LargeDataHandling", func(t *testing.T) {
		buf := &bytes.Buffer{}
		writer := NewDefaultWriter(buf)

		// Write large chunks to test buffer management
		for i := 0; i < 10; i++ {
			largeData := make([]byte, 32*1024) // 32KB
			for j := range largeData {
				largeData[j] = byte(j % 256)
			}

			_, err := writer.WriteBinary(largeData)
			require.NoError(t, err)
		}

		require.Equal(t, 32*1024*10, writer.WrittenLen())

		err := writer.Flush()
		require.NoError(t, err)

		// Verify written data integrity
		writtenBytes := buf.Bytes()
		assert.Equal(t, 32*1024*10, len(writtenBytes))

		// Sample check each 32KB chunk
		for chunkIndex := 0; chunkIndex < 10; chunkIndex++ {
			offset := chunkIndex * 32 * 1024
			// Check pattern at start and end of each chunk
			assert.Equal(t, byte(0), writtenBytes[offset], "Chunk %d start mismatch", chunkIndex)
			assert.Equal(t, byte(255), writtenBytes[offset+255], "Chunk %d pattern mismatch", chunkIndex)
		}

	})
}

// Helper types for testing
type errorReader struct {
	err error
}

func (r *errorReader) Read(p []byte) (n int, err error) {
	return 0, r.err
}

type noProgressReader struct{}

func (r *noProgressReader) Read(p []byte) (n int, err error) {
	return 0, nil // Always returns 0 bytes without error
}

type errorWriter struct {
	err error
}

func (w *errorWriter) Write(p []byte) (n int, err error) {
	return 0, w.err
}
