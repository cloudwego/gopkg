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

package apache

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

type mockReadableLen struct {
	io.ReadWriter

	n int
}

func (f *mockReadableLen) ReadableLen() int { return f.n }

func TestTBufferTransport(t *testing.T) {
	m := &mockReadableLen{n: 7}
	p := NewDefaultTransport(m)
	_ = p.IsOpen()
	_ = p.Open()
	_ = p.Close()
	_ = p.Flush(context.Background())
	require.Equal(t, uint64(7), p.RemainingBytes())
	m.n = -1
	require.Equal(t, ^uint64(0), p.RemainingBytes())

	b := &bytes.Buffer{}
	b.WriteByte(0)
	p = NewDefaultTransport(b)
	_ = p.IsOpen()
	_ = p.Open()
	_ = p.Flush(context.Background())
	require.Equal(t, uint64(1), p.RemainingBytes())
	require.NoError(t, p.Close())
	require.Equal(t, uint64(0), p.RemainingBytes())
}
