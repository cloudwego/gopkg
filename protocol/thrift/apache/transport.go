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
)

// TTransport is identical with thrift.TTransport.
type TTransport interface {
	io.ReadWriteCloser
	RemainingBytes() (num_bytes uint64)
	Flush(ctx context.Context) (err error)
	Open() error
	IsOpen() bool
}

// BufferTransport extends bytes.Buffer to support TTransport
type BufferTransport struct {
	*bytes.Buffer
}

func (p BufferTransport) IsOpen() bool                  { return true }
func (p BufferTransport) Open() error                   { return nil }
func (p BufferTransport) Close() error                  { p.Reset(); return nil }
func (p BufferTransport) Flush(_ context.Context) error { return nil }
func (p BufferTransport) RemainingBytes() uint64        { return uint64(p.Len()) }

var _ TTransport = BufferTransport{nil}
