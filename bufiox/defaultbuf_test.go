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

func TestDefaultReader_PeekReturnsBufferedOnError(t *testing.T) {
	data := []byte("Hello")
	r := NewDefaultReader(bytes.NewReader(data))

	// Peek more than available; should return buffered data + error
	buf, err := r.Peek(10)
	assert.Error(t, err)
	assert.Equal(t, data, buf)
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

func TestDefaultReader_Skip(t *testing.T) {
	t.Run("Negative", func(t *testing.T) {
		r := NewDefaultReader(bytes.NewReader(seqBytes(10)))
		assert.Equal(t, errNegativeCount, r.Skip(-1))
	})

	t.Run("Zero", func(t *testing.T) {
		r := NewDefaultReader(bytes.NewReader(seqBytes(10)))
		require.NoError(t, r.Skip(0))
		assert.Equal(t, 0, r.ReadLen())
	})

	t.Run("WithinBuffer", func(t *testing.T) {
		r := NewDefaultReader(bytes.NewReader(seqBytes(100)))
		_, err := r.Peek(1) // fill buffer
		require.NoError(t, err)

		require.NoError(t, r.Skip(10))
		assert.Equal(t, 10, r.ReadLen())
		// verify skip landed at correct position
		buf, err := r.Peek(3)
		require.NoError(t, err)
		assert.Equal(t, seqBytes(13)[10:], buf)
	})

	t.Run("BeyondBuffer", func(t *testing.T) {
		data := seqBytes(defaultBufSize + 100)
		r := NewDefaultReader(bytes.NewReader(data))

		// consume most of buffer, leaving 10 bytes
		_, err := r.Next(defaultBufSize - 10)
		require.NoError(t, err)

		// skip past buffer end: 10 buffered + 50 from reader
		require.NoError(t, r.Skip(60))
		// verify position: defaultBufSize-10 + 60
		buf, err := r.Peek(5)
		require.NoError(t, err)
		assert.Equal(t, data[defaultBufSize+50:defaultBufSize+55], buf)
	})

	t.Run("CrossRealloc", func(t *testing.T) {
		data := seqBytes(defaultBufSize * 3)
		r := NewDefaultReader(bytes.NewReader(data))

		// skip more than entire buffer capacity
		require.NoError(t, r.Skip(defaultBufSize+100))
		buf, err := r.Peek(5)
		require.NoError(t, err)
		assert.Equal(t, data[defaultBufSize+100:defaultBufSize+105], buf)
	})

	t.Run("PreservesNextData", func(t *testing.T) {
		data := seqBytes(defaultBufSize + 100)
		r := NewDefaultReader(bytes.NewReader(data))

		// hold a reference from Next
		got, err := r.Next(10)
		require.NoError(t, err)
		want := make([]byte, 10)
		copy(want, data[:10])

		// skip beyond buffer, must not corrupt got
		require.NoError(t, r.Skip(defaultBufSize))
		assert.Equal(t, want, got)
	})

	t.Run("EOF", func(t *testing.T) {
		r := NewDefaultReader(bytes.NewReader(seqBytes(10)))
		assert.Error(t, r.Skip(20))
	})
}

func seqBytes(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i)
	}
	return b
}

func TestDefaultReader_ReadBinary(t *testing.T) {
	t.Run("FromBuffer", func(t *testing.T) {
		r := NewDefaultReader(bytes.NewReader(seqBytes(100)))
		// pre-fill buffer
		_, err := r.Peek(1)
		require.NoError(t, err)

		bs := make([]byte, 10)
		n, err := r.ReadBinary(bs)
		require.NoError(t, err)
		assert.Equal(t, 10, n)
		assert.Equal(t, seqBytes(10), bs)
	})

	t.Run("SmallAcquire", func(t *testing.T) {
		// data smaller than directlyReadThreshold, triggers acquire path
		data := seqBytes(defaultBufSize + 100)
		r := NewDefaultReader(bytes.NewReader(data))

		// consume most of the buffer
		_, err := r.Next(defaultBufSize - 10)
		require.NoError(t, err)

		// need 100 bytes: 10 from buffer + 90 via acquire
		bs := make([]byte, 100)
		n, err := r.ReadBinary(bs)
		require.NoError(t, err)
		assert.Equal(t, 100, n)
		assert.Equal(t, data[defaultBufSize-10:defaultBufSize+90], bs)
	})

	t.Run("DirectRead", func(t *testing.T) {
		// remainder >= directlyReadThreshold, triggers direct readAtLeast
		data := seqBytes(defaultBufSize + directlyReadThreshold)
		r := NewDefaultReader(bytes.NewReader(data))

		// consume entire buffer
		_, err := r.Next(defaultBufSize)
		require.NoError(t, err)

		bs := make([]byte, directlyReadThreshold)
		n, err := r.ReadBinary(bs)
		require.NoError(t, err)
		assert.Equal(t, directlyReadThreshold, n)
		assert.Equal(t, data[defaultBufSize:], bs)
	})

	t.Run("EOF", func(t *testing.T) {
		r := NewDefaultReader(bytes.NewReader(seqBytes(5)))
		bs := make([]byte, 10)
		n, err := r.ReadBinary(bs)
		assert.Error(t, err)
		assert.Equal(t, 5, n)
		assert.Equal(t, seqBytes(5), bs[:5])
	})
}

func TestDefaultReader_Read(t *testing.T) {
	t.Run("FromBuffer", func(t *testing.T) {
		r := NewDefaultReader(bytes.NewReader(seqBytes(100)))
		// pre-fill buffer
		_, err := r.Peek(1)
		require.NoError(t, err)

		bs := make([]byte, 10)
		n, err := r.Read(bs)
		require.NoError(t, err)
		assert.Equal(t, 10, n)
		assert.Equal(t, seqBytes(10), bs)
	})

	t.Run("SmallAcquire", func(t *testing.T) {
		// empty buffer + small read triggers acquire(1)
		r := NewDefaultReader(bytes.NewReader(seqBytes(100)))

		bs := make([]byte, 10)
		n, err := r.Read(bs)
		require.NoError(t, err)
		assert.True(t, n > 0 && n <= 10)
		assert.Equal(t, seqBytes(n), bs[:n])
	})

	t.Run("DirectRead", func(t *testing.T) {
		// large bs >= directlyReadThreshold, triggers direct rd.Read
		data := seqBytes(directlyReadThreshold + 100)
		r := NewDefaultReader(bytes.NewReader(data))

		bs := make([]byte, directlyReadThreshold)
		n, err := r.Read(bs)
		require.NoError(t, err)
		assert.True(t, n > 0)
		assert.Equal(t, data[:n], bs[:n])
	})

	t.Run("EOF", func(t *testing.T) {
		r := NewDefaultReader(bytes.NewReader(seqBytes(3)))
		// drain all data
		_, err := r.Next(3)
		require.NoError(t, err)

		bs := make([]byte, 10)
		n, err := r.Read(bs)
		assert.Equal(t, io.EOF, err)
		assert.Equal(t, 0, n)
	})

	t.Run("IOReader", func(t *testing.T) {
		// verify Read satisfies io.Reader contract across multiple calls
		data := seqBytes(200)
		r := NewDefaultReader(bytes.NewReader(data))
		got, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Equal(t, data, got)
	})
}

func TestDefaultWriter_FlushFreesToFreeOnError(t *testing.T) {
	writeErr := errors.New("write error")
	w := NewDefaultWriter(&errorWriter{err: writeErr})

	// Malloc allocates an mcache buffer tracked in toFree
	_, err := w.Malloc(10)
	require.NoError(t, err)
	assert.NotEmpty(t, w.toFree)

	// Flush fails on WriteTo, but toFree must still be freed
	err = w.Flush()
	assert.Equal(t, writeErr, err)

	// toFree should be drained even on error
	for _, buf := range w.toFree {
		assert.Nil(t, buf, "toFree buffer not freed after Flush error")
	}
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
