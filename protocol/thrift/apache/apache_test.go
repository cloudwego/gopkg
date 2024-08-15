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
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestThriftReadWrite(t *testing.T) {

	v := &TestingWriteRead{Msg: "Hello"}

	err := CheckTStruct(v)
	require.Same(t, err, errCheckTStructNotRegistered)
	RegisterCheckTStruct(checkTStruct)
	err = CheckTStruct(v)
	require.NoError(t, err)

	buf := &bytes.Buffer{}

	err = ThriftWrite(buf, v)
	require.Same(t, err, errThriftWriteNotRegistered)

	RegisterThriftWrite(callThriftWrite)
	err = ThriftWrite(NewBufferTransport(buf), v) // calls v.Write
	require.NoError(t, err)

	p := &TestingWriteRead{}

	err = ThriftRead(NewBufferTransport(buf), p)
	require.Same(t, err, errThriftReadNotRegistered)

	RegisterThriftRead(callThriftRead)
	err = ThriftRead(NewBufferTransport(buf), p) // calls p.Read
	require.NoError(t, err)

	require.Equal(t, v.Msg, p.Msg)
}

type TStruct interface { // simulate thrift.TStruct
	Read(r io.Reader) error
	Write(w io.Writer) error
}

type TestingWriteRead struct {
	Msg string
}

func (t *TestingWriteRead) Read(r io.Reader) error {
	return json.NewDecoder(r).Decode(t)
}

func (t *TestingWriteRead) Write(w io.Writer) error {
	return json.NewEncoder(w).Encode(t)
}

var errNotThriftTStruct = errors.New("errNotThriftTStruct")

func checkTStruct(v interface{}) error {
	_, ok := v.(TStruct)
	if !ok {
		return errNotThriftTStruct
	}
	return nil
}

func callThriftRead(rw io.ReadWriter, v interface{}) error {
	p, ok := v.(TStruct)
	if !ok {
		return errNotThriftTStruct
	}
	return p.Read(rw)
}

func callThriftWrite(rw io.ReadWriter, v interface{}) error {
	p, ok := v.(TStruct)
	if !ok {
		return errNotThriftTStruct
	}
	return p.Write(rw)
}
