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
	"math"
	"sync"
)

const defaultBinaryWriterBufferSize = 4096

type BinaryWriter struct {
	buf []byte
}

var poolBinaryWriter = sync.Pool{
	New: func() interface{} {
		return &BinaryWriter{buf: make([]byte, 0, defaultBinaryWriterBufferSize)}
	},
}

func NewBinaryWriter() *BinaryWriter {
	return NewBinaryWriterSize(0)
}

func NewBinaryWriterSize(sz int) *BinaryWriter {
	w := poolBinaryWriter.Get().(*BinaryWriter)
	if cap(w.buf) < sz {
		w.Release()
		w = &BinaryWriter{buf: make([]byte, 0, sz)}
	}
	w.Reset()
	return w
}

func (w *BinaryWriter) Release() {
	poolBinaryWriter.Put(w)
}

func (w *BinaryWriter) Reset() {
	w.buf = w.buf[:0]
}

func (w *BinaryWriter) Bytes() []byte {
	return w.buf
}

func (w *BinaryWriter) WriteMessageBegin(name string, typeID TMessageType, seq int32) {
	w.buf = Binary.AppendMessageBegin(w.buf, name, typeID, seq)
}

func (w *BinaryWriter) WriteFieldBegin(typeID TType, id int16) {
	w.buf = Binary.AppendFieldBegin(w.buf, typeID, id)
}

func (w *BinaryWriter) WriteFieldStop() {
	w.buf = append(w.buf, byte(STOP))
}

func (w *BinaryWriter) WriteMapBegin(kt, vt TType, size int) {
	w.buf = Binary.AppendMapBegin(w.buf, kt, vt, size)
}

func (w *BinaryWriter) WriteListBegin(et TType, size int) {
	w.buf = Binary.AppendListBegin(w.buf, et, size)
}

func (w *BinaryWriter) WriteSetBegin(et TType, size int) {
	w.buf = Binary.AppendSetBegin(w.buf, et, size)
}

func (w *BinaryWriter) WriteBinary(v []byte) {
	w.buf = Binary.AppendBinary(w.buf, v)
}

func (w *BinaryWriter) WriteString(v string) {
	w.buf = Binary.AppendString(w.buf, v)
}

func (w *BinaryWriter) WriteBool(v bool) {
	if v {
		w.buf = append(w.buf, 1)
	} else {
		w.buf = append(w.buf, 0)
	}
}

func (w *BinaryWriter) WriteByte(v int8) {
	w.buf = append(w.buf, byte(v))
}

func (w *BinaryWriter) WriteI16(v int16) {
	w.buf = append(w.buf, byte(uint16(v)>>8), byte(v))
}

func (w *BinaryWriter) WriteI32(v int32) {
	w.buf = appendUint32(w.buf, uint32(v))
}

func (w *BinaryWriter) WriteI64(v int64) {
	w.buf = appendUint64(w.buf, uint64(v))
}

func (w *BinaryWriter) WriteDouble(v float64) {
	w.buf = appendUint64(w.buf, math.Float64bits(v))
}
