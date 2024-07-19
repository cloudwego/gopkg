/*
 * Copyright 2024 CloudWeGo Authors
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

package thrift

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSkipReader(t *testing.T) {
	b := make([]byte, 2048)
	for i := 0; i < len(b); i++ {
		b[i] = byte(i)
	}

	r := newSkipReader(bytes.NewReader(b))
	defer r.Release()
	for i := 0; i < len(b); i++ {
		b, err := r.Next(1)
		require.NoError(t, err)
		require.True(t, b[0] == byte(i))
	}

	retb, err := r.Bytes()
	require.NoError(t, err)
	require.Equal(t, b, retb)
}

type remoteByteBufferImplForT struct {
	p int
	b []byte
}

func (remoteByteBufferImplForT) Read(_ []byte) (int, error) { return 0, errors.New("not implemented") }

func (p *remoteByteBufferImplForT) Peek(n int) (buf []byte, err error) {
	if n > len(p.b) {
		return nil, io.EOF
	}
	return p.b[:n], nil
}

func (p *remoteByteBufferImplForT) ReadableLen() int {
	return len(p.b) - p.p
}

func (p *remoteByteBufferImplForT) Skip(n int) error {
	if n > len(p.b) {
		panic("bug")
	}
	p.p += n
	return nil
}

func TestSkipRemoteBuffer(t *testing.T) {
	b := make([]byte, 2048)
	for i := 0; i < len(b); i++ {
		b[i] = byte(i)
	}

	r := newSkipByteBuffer(&remoteByteBufferImplForT{b: b})
	defer r.Release()
	for i := 0; i < len(b); i++ {
		b, err := r.Next(1)
		require.NoError(t, err)
		require.True(t, b[0] == byte(i))
	}

	retb, err := r.Bytes()
	require.NoError(t, err)
	require.Equal(t, b, retb)
}
