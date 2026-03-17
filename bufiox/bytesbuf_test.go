// Copyright 2025 CloudWeGo Authors
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

	"github.com/cloudwego/gopkg/internal/assert"
)

func TestBytesReader_BasicFunctionality(t *testing.T) {
	data := []byte("Hello, BytesReader!")
	reader := NewBytesReader(data)

	buf, err := reader.Next(5)
	assert.Nil(t, err)
	assert.Equal(t, "Hello", string(buf))
	assert.Equal(t, 5, reader.ReadLen())

	peekBuf, err := reader.Peek(1)
	assert.Nil(t, err)
	assert.Equal(t, ",", string(peekBuf))
	assert.Equal(t, 5, reader.ReadLen())

	err = reader.Skip(1)
	assert.Nil(t, err)
	assert.Equal(t, 6, reader.ReadLen())

	buf, err = reader.Next(6)
	assert.Nil(t, err)
	assert.Equal(t, " Bytes", string(buf))

	var binaryBuf [5]byte
	n, err := reader.ReadBinary(binaryBuf[:])
	assert.Nil(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, "Reade", string(binaryBuf[:]))

	err = reader.Release(nil)
	assert.Nil(t, err)
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
		assert.Nil(t, err)
		assert.Equal(t, 0, len(buf))

		buf, err = reader.Peek(0)
		assert.Nil(t, err)
		assert.Equal(t, 0, len(buf))

		err = reader.Skip(0)
		assert.Nil(t, err)
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
		assert.Nil(t, err)
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
		assert.Nil(t, err)
		assert.Equal(t, "012", string(buf))

		peekBuf, err := reader.Peek(3)
		assert.Nil(t, err)
		assert.Equal(t, "345", string(peekBuf))

		assert.Equal(t, 3, reader.ReadLen())

		buf, err = reader.Next(3)
		assert.Nil(t, err)
		assert.Equal(t, "345", string(buf))
		assert.Equal(t, 6, reader.ReadLen())
	})

	t.Run("SkipAfterPeek", func(t *testing.T) {
		peekBuf, err := reader.Peek(2)
		assert.Nil(t, err)
		assert.Equal(t, "67", string(peekBuf))

		err = reader.Skip(2)
		assert.Nil(t, err)
		assert.Equal(t, 8, reader.ReadLen())

		buf, err := reader.Next(2)
		assert.Nil(t, err)
		assert.Equal(t, "89", string(buf))
		assert.Equal(t, 10, reader.ReadLen())
	})

	t.Run("PartialReadBinary", func(t *testing.T) {
		reader := NewBytesReader(data)

		var buf [5]byte
		n, err := reader.ReadBinary(buf[:3])
		assert.Nil(t, err)
		assert.Equal(t, 3, n)
		assert.Equal(t, "012", string(buf[:3]))

		n, err = reader.ReadBinary(buf[:])
		assert.Nil(t, err)
		assert.Equal(t, 5, n)
		assert.Equal(t, "34567", string(buf[:]))
	})
}

// TestBytesWriter_BasicFunctionality tests basic BytesWriter functionality
func TestBytesWriter_BasicFunctionality(t *testing.T) {
	var buf []byte
	writer := NewBytesWriter(&buf)

	mallocBuf, err := writer.Malloc(10)
	assert.Nil(t, err)
	assert.Equal(t, 10, len(mallocBuf))
	copy(mallocBuf, []byte("0123456789"))
	assert.Equal(t, 10, writer.WrittenLen())

	n, err := writer.WriteBinary([]byte("Hello"))
	assert.Nil(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, 15, writer.WrittenLen())

	err = writer.Flush()
	assert.Nil(t, err)
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
		assert.Nil(t, err)
		assert.Equal(t, 0, len(mallocBuf))
		assert.Equal(t, 0, writer.WrittenLen())
	})

	t.Run("EmptyWrite", func(t *testing.T) {
		var emptyBuf []byte
		writer := NewBytesWriter(&emptyBuf)

		n, err := writer.WriteBinary([]byte{})
		assert.Nil(t, err)
		assert.Equal(t, 0, n)
		assert.Equal(t, 0, writer.WrittenLen())

		err = writer.Flush()
		assert.Nil(t, err)
		assert.Equal(t, 0, len(emptyBuf))
	})

	t.Run("FlushWithoutData", func(t *testing.T) {
		var flushBuf []byte
		writer := NewBytesWriter(&flushBuf)

		err := writer.Flush()
		assert.Nil(t, err)
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
		assert.Nil(t, err)
		assert.Equal(t, len(largeData), n)
		assert.Equal(t, len(largeData), writer.WrittenLen())

		err = writer.Flush()
		assert.Nil(t, err)
		assert.Equal(t, len(largeData), len(buf))
		assert.BytesEqual(t, largeData, buf)
	})

	t.Run("MultipleMalloc", func(t *testing.T) {
		var buf []byte
		writer := NewBytesWriter(&buf)

		// Multiple small mallocs
		for i := 0; i < 10; i++ {
			mallocBuf, err := writer.Malloc(10)
			assert.Nil(t, err)
			copy(mallocBuf, []byte("0123456789"))
		}

		assert.Equal(t, 100, writer.WrittenLen())

		err := writer.Flush()
		assert.Nil(t, err)
		assert.Equal(t, 100, len(buf))
		assert.Equal(t, "0123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789", string(buf))
	})

	t.Run("MixedOperations", func(t *testing.T) {
		var buf []byte
		writer := NewBytesWriter(&buf)

		// Mix of Malloc and WriteBinary operations
		mallocBuf, err := writer.Malloc(5)
		assert.Nil(t, err)
		copy(mallocBuf, []byte("Hello"))

		n, err := writer.WriteBinary([]byte("World"))
		assert.Nil(t, err)
		assert.Equal(t, 5, n)

		mallocBuf, err = writer.Malloc(1)
		assert.Nil(t, err)
		copy(mallocBuf, "!")

		assert.Equal(t, 11, writer.WrittenLen())

		err = writer.Flush()
		assert.Nil(t, err)
		assert.Equal(t, "HelloWorld!", string(buf))
	})
}

// TestBytesReader_ReleaseBehavior tests Release behavior
func TestBytesReader_ReleaseBehavior(t *testing.T) {
	data := []byte("0123456789")
	reader := NewBytesReader(data)

	// Read some data
	buf, err := reader.Next(3)
	assert.Nil(t, err)
	assert.Equal(t, "012", string(buf))
	assert.Equal(t, 3, reader.ReadLen())

	// Release and check behavior
	err = reader.Release(nil)
	assert.Nil(t, err)
	assert.Equal(t, 0, reader.ReadLen())

	remainingBuf, err := reader.Next(7)
	assert.Nil(t, err)
	assert.Equal(t, "3456789", string(remainingBuf))
	assert.Equal(t, 7, reader.ReadLen())

	err = reader.Release(nil)
	assert.Nil(t, err)
	assert.Equal(t, 0, reader.ReadLen())
}

// TestBytesReaderAndWriter_Interaction tests interaction between BytesReader and BytesWriter
func TestBytesReaderAndWriter_Interaction(t *testing.T) {
	originalData := []byte("Hello, World!")

	// Write data using BytesWriter
	var buf []byte
	writer := NewBytesWriter(&buf)

	n, err := writer.WriteBinary(originalData)
	assert.Nil(t, err)
	assert.Equal(t, len(originalData), n)

	err = writer.Flush()
	assert.Nil(t, err)
	assert.BytesEqual(t, originalData, buf)

	// Read data using BytesReader
	reader := NewBytesReader(buf)

	readData := make([]byte, len(originalData))
	n, err = reader.ReadBinary(readData)
	assert.Nil(t, err)
	assert.Equal(t, len(originalData), n)
	assert.BytesEqual(t, originalData, readData)

	assert.Equal(t, len(originalData), reader.ReadLen())

	_, err = reader.Next(1)
	assert.Equal(t, errNoRemainingData, err)

	err = reader.Release(nil)
	assert.Nil(t, err)
	assert.Equal(t, 0, reader.ReadLen())
}

// TestBytesReader_ErrorConsistency tests that BytesReader returns consistent errors
func TestBytesReader_ErrorConsistency(t *testing.T) {
	data := []byte("test")
	reader := NewBytesReader(data)

	// Read all data
	buf, err := reader.Next(len(data))
	assert.Nil(t, err)
	assert.BytesEqual(t, data, buf)

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
	assert.Nil(t, err)

	err = writer.Flush()
	assert.Nil(t, err)
	assert.Equal(t, "Hello", string(buf))

	err = writer.Flush()
	assert.Nil(t, err)
	assert.Equal(t, "Hello", string(buf))

	m, err := writer.Malloc(1)
	assert.Nil(t, err)
	m[0] = '!'

	err = writer.Flush()
	assert.Nil(t, err)
	assert.Equal(t, "Hello!", string(buf))
}

func TestBytesWriter_PreExistingData(t *testing.T) {
	t.Run("UseExistingCap", func(t *testing.T) {
		buf := make([]byte, 5, 100)
		copy(buf, "Hello")
		w := NewBytesWriter(&buf)
		assert.Equal(t, 0, w.WrittenLen())

		_, err := w.WriteBinary([]byte("World"))
		assert.Nil(t, err)

		err = w.Flush()
		assert.Nil(t, err)
		assert.Equal(t, "HelloWorld", string(buf))
	})

	t.Run("GrowPreservesData", func(t *testing.T) {
		buf := make([]byte, 5, 10)
		copy(buf, "Hello")
		w := NewBytesWriter(&buf)

		m, err := w.Malloc(20)
		assert.Nil(t, err)
		copy(m, "WorldAndMoreStuff!!!")

		err = w.Flush()
		assert.Nil(t, err)
		assert.Equal(t, "HelloWorldAndMoreStuff!!!", string(buf))
	})

	t.Run("MallocDeferredCopy", func(t *testing.T) {
		buf := make([]byte, 5, 10)
		copy(buf, "Hello")
		w := NewBytesWriter(&buf)

		// m1 within existing cap
		m1, _ := w.Malloc(3)

		// m2 triggers grow; m1 now points to oldBuf
		m2, _ := w.Malloc(20)

		// write AFTER grow to verify deferred copy
		copy(m1, "AB!")
		copy(m2, "CDE_extra_data_here!")

		err := w.Flush()
		assert.Nil(t, err)
		assert.Equal(t, "HelloAB!CDE_extra_data_here!", string(buf))
	})
}

func TestBytesWriter_FlushGrowFlush(t *testing.T) {
	var buf []byte
	w := NewBytesWriter(&buf)

	// first write + flush
	_, err := w.WriteBinary([]byte("Hello"))
	assert.Nil(t, err)
	err = w.Flush()
	assert.Nil(t, err)
	assert.Equal(t, "Hello", string(buf))

	// write enough to trigger acquireSlow (defaultBufSize = 8KB)
	big := make([]byte, 16*1024)
	for i := range big {
		big[i] = 'X'
	}
	_, err = w.WriteBinary(big)
	assert.Nil(t, err)

	// second flush must reconstruct pre-flush data via oldBuf
	err = w.Flush()
	assert.Nil(t, err)

	want := "Hello" + string(big)
	assert.Equal(t, want, string(buf))
}

// TestBytesWriter_AcquireSlowCoverage tests acquireSlow function branches
func TestBytesWriter_AcquireSlowCoverage(t *testing.T) {
	t.Run("InitialAllocation", func(t *testing.T) {
		var buf []byte
		writer := NewBytesWriter(&buf)

		mallocBuf, err := writer.Malloc(16 * 1024)
		assert.Nil(t, err)
		assert.Equal(t, 16*1024, len(mallocBuf))

		_, err = writer.WriteBinary(make([]byte, 32*1024))
		assert.Nil(t, err)

		err = writer.Flush()
		assert.Nil(t, err)
		assert.Equal(t, 48*1024, len(buf))
	})

	t.Run("ExistingBufferGrowth", func(t *testing.T) {
		var buf []byte
		writer := NewBytesWriter(&buf)

		_, err := writer.WriteBinary([]byte("initial"))
		assert.Nil(t, err)

		mallocBuf, err := writer.Malloc(16 * 1024)
		assert.Nil(t, err)
		assert.Equal(t, 16*1024, len(mallocBuf))

		for i := 0; i < len(mallocBuf); i++ {
			mallocBuf[i] = byte(i % 256)
		}

		err = writer.Flush()
		assert.Nil(t, err)
		assert.True(t, len(buf) > 16*1024)
	})
}
