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
	"testing"

	"github.com/bytedance/gopkg/lang/dirtmake"
	"github.com/cloudwego/gopkg/bufiox"
	"github.com/stretchr/testify/require"
)

const defaultBinaryWriterBufferSize = 4096

func TestBinaryWriter(t *testing.T) {
	buf := dirtmake.Bytes(0, defaultBinaryWriterBufferSize*2)
	w := NewBufferWriter(bufiox.NewBytesWriter(&buf))
	x := BinaryProtocol{}

	b := x.AppendMessageBegin(nil, "hello", 1, 2)
	w.WriteMessageBegin("hello", 1, 2)

	b = x.AppendFieldBegin(b, 3, 4)
	w.WriteFieldBegin(3, 4)

	b = x.AppendFieldStop(b)
	w.WriteFieldStop()

	b = x.AppendMapBegin(b, 5, 6, 7)
	w.WriteMapBegin(5, 6, 7)

	b = x.AppendListBegin(b, 8, 9)
	w.WriteListBegin(8, 9)

	b = x.AppendSetBegin(b, 10, 11)
	w.WriteSetBegin(10, 11)

	b = x.AppendBinary(b, []byte("12"))
	w.WriteBinary([]byte("12"))

	b = x.AppendString(b, "13")
	w.WriteString("13")

	b = x.AppendBool(b, true)
	b = x.AppendBool(b, false)
	w.WriteBool(true)
	w.WriteBool(false)

	b = x.AppendByte(b, 14)
	w.WriteByte(14)

	b = x.AppendI16(b, 15)
	w.WriteI16(15)

	b = x.AppendI32(b, 16)
	w.WriteI32(16)

	b = x.AppendI64(b, 17)
	w.WriteI64(17)

	b = x.AppendDouble(b, 18.5)
	w.WriteDouble(18.5)

	w.w.Flush()
	w.Recycle()

	require.Equal(t, b, buf)
}
