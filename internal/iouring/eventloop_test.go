/*
 * Copyright 2025 CloudWeGo Authors
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

package iouring

import (
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewIOUringEventLoop(t *testing.T) {
	skipIfUnsupported(t)

	cfg := DefaultConfig()
	evl, err := NewIOUringEventLoop(cfg)
	require.NoError(t, err)
	require.NotNil(t, evl)
	require.NotNil(t, evl.ring)
	require.NotNil(t, evl.ring.r)
}

func TestEventLoopReadWrite(t *testing.T) {
	skipIfUnsupported(t)

	cfg := DefaultConfig()
	cfg.SQEBatchSize = 1 // Submit immediately
	evl, err := NewIOUringEventLoop(cfg)
	require.NoError(t, err)

	c := createConnections(t, 1)[0]
	defer c.Close()

	// Test write operation with 1MB buffer
	testData := make([]byte, 1024*1024) // 1MB
	// Fill with test pattern
	for i := range testData {
		testData[i] = byte(i % 256)
	}
	ud := userDataPoolGet()
	defer userDataPoolPut(ud)

	ud.SetWriteOp(int32(getFd(t, c.client)), testData)

	evl.ring.sqeChan <- ud

	// Verify data received using io.ReadFull
	readBuf := make([]byte, 1024*1024) // 1MB buffer
	n, err := io.ReadFull(c.server, readBuf)
	require.NoError(t, err)
	require.Equal(t, len(testData), n)

	// Verify data pattern matches
	for i := 0; i < len(testData); i++ {
		if readBuf[i] != byte(i%256) {
			t.Fatalf("data mismatch at byte %d: expected %d, got %d", i, byte(i%256), readBuf[i])
		}
	}

	// Verify write completed (move to end to avoid blocking)
	res := ud.Wait()
	require.Equal(t, int32(len(testData)), res)
}

func TestBatchSubmit(t *testing.T) {
	skipIfUnsupported(t)

	cfg := DefaultConfig()
	cfg.SQEBatchSize = 3
	cfg.SQESubmitInterval = 0 // disable timer

	evl, err := NewIOUringEventLoop(cfg)
	require.NoError(t, err)

	conns := createConnections(t, 3)
	defer func() {
		for _, c := range conns {
			c.Close()
		}
	}()

	// Submit 3 writes to trigger batch submit
	testData := []byte("batch test")
	uds := make([]*userData, 3)

	for i := 0; i < 3; i++ {
		uds[i] = userDataPoolGet()
		uds[i].SetWriteOp(int32(getFd(t, conns[i].client)), testData)
		evl.ring.sqeChan <- uds[i]
	}

	for i := 0; i < 3; i++ {
		res := uds[i].Wait()
		userDataPoolPut(uds[i])
		require.Equal(t, int32(len(testData)), res)
	}
}

func TestTimerSubmit(t *testing.T) {
	skipIfUnsupported(t)

	cfg := DefaultConfig()
	cfg.SQEBatchSize = 100
	cfg.SQESubmitInterval = 20 * time.Millisecond

	evl, err := NewIOUringEventLoop(cfg)
	require.NoError(t, err)

	c := createConnections(t, 1)[0]
	defer c.Close()

	// Submit single write, should be submitted by timer
	testData := []byte("timer test")
	ud := userDataPoolGet()
	defer userDataPoolPut(ud)

	ud.SetWriteOp(int32(getFd(t, c.client)), testData)

	evl.ring.sqeChan <- ud
	res := ud.Wait()
	require.Equal(t, int32(len(testData)), res)
}
