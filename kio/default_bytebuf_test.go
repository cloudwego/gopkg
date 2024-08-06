/*
 * Copyright 2021 CloudWeGo Authors
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

package kio

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultByteBuffer(t *testing.T) {
	buf1 := NewReaderWriterBuffer(-1)
	checkWritable(t, buf1)
	checkReadable(t, buf1)

	buf2 := NewReaderWriterBuffer(1024)
	checkWritable(t, buf2)
	checkReadable(t, buf2)

	buf3 := buf2.NewBuffer()
	checkWritable(t, buf3)
	checkUnreadable(t, buf3)

	buf4 := NewReaderWriterBuffer(-1)
	_, err := buf3.Bytes()
	assert.True(t, err == nil, err)
	err = buf4.AppendBuffer(buf3)
	assert.True(t, err == nil)
	checkReadable(t, buf4)
}

func TestDefaultWriterBuffer(t *testing.T) {
	buf := NewWriterBuffer(-1)
	checkWritable(t, buf)
	checkUnreadable(t, buf)
}

func TestDefaultReaderBuffer(t *testing.T) {
	msg := "hello world"
	b := []byte(msg + msg + msg + msg + msg)
	buf := NewReaderBuffer(b)
	checkUnwritable(t, buf)
	checkReadable(t, buf)
}

func checkWritable(t *testing.T, buf ByteBuffer) {
	msg := "hello world"

	p, err := buf.Malloc(len(msg))
	assert.True(t, err == nil, err)
	assert.True(t, len(p) == len(msg))
	copy(p, msg)
	l := buf.MallocLen()
	assert.True(t, l == len(msg))
	l, err = buf.WriteString(msg)
	assert.True(t, err == nil, err)
	assert.True(t, l == len(msg))
	l, err = buf.WriteBinary([]byte(msg))
	assert.True(t, err == nil, err)
	assert.True(t, l == len(msg))
	l, err = buf.Write([]byte(msg))
	assert.True(t, err == nil, err)
	assert.True(t, l == len(msg))
	err = buf.Flush()
	assert.True(t, err == nil, err)
	var n int
	n, err = buf.Write([]byte(msg))
	assert.True(t, err == nil, err)
	assert.True(t, n == len(msg))
	b, err := buf.Bytes()
	assert.True(t, err == nil, err)
	assert.True(t, string(b) == msg+msg+msg+msg+msg, string(b))
}

func checkReadable(t *testing.T, buf ByteBuffer) {
	msg := "hello world"

	p, err := buf.Peek(len(msg))
	assert.True(t, err == nil, err)
	assert.True(t, string(p) == msg)
	err = buf.Skip(1 + len(msg))
	assert.True(t, err == nil, err)
	p, err = buf.Next(len(msg) - 1)
	assert.True(t, err == nil, err)
	assert.True(t, string(p) == msg[1:])
	n := buf.ReadableLen()
	assert.True(t, n == 3*len(msg), n)
	n = buf.ReadLen()
	assert.True(t, n == 2*len(msg), n)

	var s string
	s, err = buf.ReadString(len(msg))
	assert.True(t, err == nil, err)
	assert.True(t, s == msg)
	p, err = buf.ReadBinary(len(msg))
	assert.True(t, err == nil, err)
	assert.True(t, string(p) == msg)
	p = make([]byte, len(msg))
	n, err = buf.Read(p)
	assert.True(t, err == nil, err)
	assert.True(t, string(p) == msg)
	assert.True(t, n == 11, n)
}

func checkUnwritable(t *testing.T, buf ByteBuffer) {
	msg := "hello world"
	_, err := buf.Malloc(len(msg))
	assert.True(t, err != nil)
	l := buf.MallocLen()
	assert.True(t, l == -1, l)
	_, err = buf.WriteString(msg)
	assert.True(t, err != nil)
	_, err = buf.WriteBinary([]byte(msg))
	assert.True(t, err != nil)
	err = buf.Flush()
	assert.True(t, err != nil)
	var n int
	n, err = buf.Write([]byte(msg))
	assert.True(t, err != nil)
	assert.True(t, n == -1, n)
}

func checkUnreadable(t *testing.T, buf ByteBuffer) {
	msg := "hello world"
	_, err := buf.Peek(len(msg))
	assert.True(t, err != nil)
	err = buf.Skip(1)
	assert.True(t, err != nil)
	_, err = buf.Next(len(msg) - 1)
	assert.True(t, err != nil)
	n := buf.ReadableLen()
	assert.True(t, n == -1)
	n = buf.ReadLen()
	assert.True(t, n == 0)
	_, err = buf.ReadString(len(msg))
	assert.True(t, err != nil)
	_, err = buf.ReadBinary(len(msg))
	assert.True(t, err != nil)
	p := make([]byte, len(msg))
	n, err = buf.Read(p)
	assert.True(t, err != nil)
	assert.True(t, n == -1, n)
}
