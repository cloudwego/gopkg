/*
 * Copyright 2026 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package bufiox

import (
	"bytes"
	"crypto/rand"
	"io"
	"net"
	"testing"
)

// generateTestData generates random test data of specified size
func generateTestData(size int) []byte {
	data := make([]byte, size)
	_, _ = rand.Read(data)
	return data
}

// createNetConn creates a real network connection for testing
func createNetConn() (net.Conn, net.Conn, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, err
	}

	var serverConn net.Conn
	var acceptErr error

	done := make(chan struct{})
	go func() {
		serverConn, acceptErr = listener.Accept()
		close(done)
	}()

	clientConn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		listener.Close()
		return nil, nil, err
	}

	<-done
	if acceptErr != nil {
		clientConn.Close()
		listener.Close()
		return nil, nil, acceptErr
	}

	listener.Close()
	return serverConn, clientConn, nil
}

func benchmarkDefaultReaderDirectRead(b *testing.B, size int) {
	data := generateTestData(size)
	buf := make([]byte, size)
	b.ResetTimer()
	b.SetBytes(int64(size))

	for i := 0; i < b.N; i++ {
		// Create a new reader for each iteration to avoid data exhaustion
		reader := NewDefaultReader(bytes.NewReader(data))

		// Complete flow: ReadBinary + Release
		_, err := reader.ReadBinary(buf)
		if err != nil && err != io.EOF {
			b.Fatal(err)
		}

		err = reader.Release(nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDefaultReaderDirectRead_Small tests direct read optimization for small packets
// Small packet (2KB < 4KB): read via buffer
// Complete flow: ReadBinary -> Release
func BenchmarkDefaultReaderDirectRead_Small(b *testing.B) {
	size := 2 * 1024 // 2KB
	benchmarkDefaultReaderDirectRead(b, size)
}

// BenchmarkDefaultReader_DirectRead_Large tests direct read optimization for large packets
// Large packet (8KB > 4KB): direct read
// Complete flow: ReadBinary -> Release
func BenchmarkDefaultReaderDirectRead_Large(b *testing.B) {
	size := 8 * 1024 // 8KB
	benchmarkDefaultReaderDirectRead(b, size)
}

func benchmarkDefaultWriterWriteBinary(b *testing.B, size int) {
	data := generateTestData(size)

	b.ResetTimer()
	b.SetBytes(int64(size))

	for i := 0; i < b.N; i++ {
		writer := NewDefaultWriter(bytes.NewBuffer(nil))
		// Complete flow: WriteBinary + Flush
		_, err := writer.WriteBinary(data)
		if err != nil {
			b.Fatal(err)
		}

		err = writer.Flush()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDefaultWriter_WriteBinary_Small tests small packet writing
// Small packet (2KB < 4KB): write to internal buffer
// Complete flow: WriteBinary -> Flush
func BenchmarkDefaultWriter_WriteBinary_Small(b *testing.B) {
	size := 2 * 1024 // 2KB
	benchmarkDefaultWriterWriteBinary(b, size)
}

// BenchmarkDefaultWriter_WriteBinary_Large tests large packet writing
// Large packet (8KB > 4KB): zero-copy write
// Complete flow: WriteBinary -> Flush
func BenchmarkDefaultWriter_WriteBinary_Large(b *testing.B) {
	size := 8 * 1024 // 8KB
	benchmarkDefaultWriterWriteBinary(b, size)
}

// BenchmarkDefaultWriter_WriteV_MultiChunk tests writev system call
// Multiple small chunk writes, accumulating chunks
// Complete flow: multiple WriteBinary -> Flush (triggers writev)
func BenchmarkDefaultWriter_WriteV_MultiChunk(b *testing.B) {
	chunkSize := 1024 * 4
	chunkCount := 8
	totalSize := chunkSize * chunkCount
	chunks := make([][]byte, chunkCount)
	for i := 0; i < chunkCount; i++ {
		chunks[i] = generateTestData(chunkSize)
	}

	serverConn, clientConn, err := createNetConn()
	if err != nil {
		b.Fatal(err)
	}
	defer serverConn.Close()
	defer clientConn.Close()
	buf := make([]byte, totalSize)
	writer := NewDefaultWriter(clientConn)

	b.ResetTimer()
	b.SetBytes(int64(totalSize))

	for i := 0; i < b.N; i++ {
		// Write multiple small data chunks
		for _, chunk := range chunks {
			_, err := writer.WriteBinary(chunk)
			if err != nil {
				b.Fatal(err)
			}
		}

		// Flush once to trigger writev
		err = writer.Flush()
		if err != nil {
			b.Fatal(err)
		}

		// Read data to avoid blocking
		_, _ = serverConn.Read(buf)
	}
}

// BenchmarkDefaultReader_AdaptiveBuffer tests adaptive buffer
// Mixed-size reads to trigger adaptive adjustment
// Complete flow: multiple ReadBinary of different sizes -> Release (logs cumulative read bytes)
func BenchmarkDefaultReader_AdaptiveBuffer(b *testing.B) {
	sizes := []int{1024, 2048, 4096, 8192, 16384, 32768} // 1KB to 32KB
	dataList := make([][]byte, len(sizes))
	for i, size := range sizes {
		dataList[i] = generateTestData(size)
	}

	serverConn, clientConn, err := createNetConn()
	if err != nil {
		b.Fatal(err)
	}
	defer serverConn.Close()
	defer clientConn.Close()

	reader := NewDefaultReader(serverConn)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		data := dataList[i%len(dataList)]

		_, err = clientConn.Write(data)
		if err != nil {
			b.Fatal(err)
		}

		_, err = reader.Next(len(data))
		if err != nil {
			b.Fatal(err)
		}

		err := reader.Release(nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}
