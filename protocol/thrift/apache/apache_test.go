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
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterNewTBinaryProtocol(t *testing.T) {
	{ // case: not func type
		fn := 1
		err := RegisterNewTBinaryProtocol(fn)
		t.Log(err)
		assert.ErrorIs(t, err, errNewFuncType)
	}

	{ // case: args err
		fn := func(_ TTransport, _ bool, _ int) {}
		err := RegisterNewTBinaryProtocol(fn)
		t.Log(err)
		assert.ErrorIs(t, err, errNewFuncType)
	}

	{ // case: ret err
		fn := func(_ TTransport, _, _ bool) {}
		err := RegisterNewTBinaryProtocol(fn)
		t.Log(err)
		assert.ErrorIs(t, err, errNewFuncType)
	}

	{ // case: no err
		fn := func(_ TTransport, _, _ bool) error { return nil }
		err := RegisterNewTBinaryProtocol(fn)
		assert.NoError(t, err)
		assert.True(t, newTBinaryProtocol.IsValid())
		newTBinaryProtocol = reflect.Value{} // reset
	}
}

type TestingWriteRead struct {
	Msg string

	mockErr error
}

func (t *TestingWriteRead) Read(r io.Reader) error {
	if t.mockErr != nil {
		return t.mockErr
	}
	return json.NewDecoder(r).Decode(t)
}

func (t *TestingWriteRead) Write(w io.Writer) error {
	if t.mockErr != nil {
		return t.mockErr
	}
	return json.NewEncoder(w).Encode(t)
}

func TestThriftWriteRead(t *testing.T) {
	called := 0
	fn := func(trans TTransport, b0, b1 bool) *bytes.Buffer {
		assert.True(t, b0)
		assert.True(t, b1)
		called++
		return trans.(BufferTransport).Buffer
	}
	err := RegisterNewTBinaryProtocol(fn)
	require.NoError(t, err)
	defer func() { newTBinaryProtocol = reflect.Value{} }()

	buf := &bytes.Buffer{}
	p0 := &TestingWriteRead{Msg: "hello"}
	err = ThriftWrite(BufferTransport{buf}, p0) // calls p0.Write
	require.NoError(t, err)
	require.Equal(t, 1, called)

	p1 := &TestingWriteRead{}
	err = ThriftRead(BufferTransport{buf}, p1) // calls p1.Read
	require.NoError(t, err)
	require.Equal(t, 2, called)
	require.Equal(t, p0, p1)
}

type TestingWriteReadMethodNotMatch struct{}

func (p *TestingWriteReadMethodNotMatch) Read(v bool) error  { return nil }
func (p *TestingWriteReadMethodNotMatch) Write(v bool) error { return nil }

func TestThriftWriteReadErr(t *testing.T) {
	var err error

	// errNotPointer
	p := TestingWriteRead{Msg: "hello"}
	err = ThriftWrite(BufferTransport{nil}, p)
	assert.Same(t, err, errNotPointer)
	err = ThriftRead(BufferTransport{nil}, p)
	assert.Same(t, err, errNotPointer)

	// errNoNewTBinaryProtocol
	err = ThriftWrite(BufferTransport{nil}, &p)
	assert.Same(t, err, errNoNewTBinaryProtocol)

	// Read/Write returns err
	fn := func(trans TTransport, b0, b1 bool) *bytes.Buffer { return nil }
	RegisterNewTBinaryProtocol(fn)
	defer func() { newTBinaryProtocol = reflect.Value{} }()
	p.mockErr = errors.New("mock")
	err = ThriftWrite(BufferTransport{nil}, &p)
	assert.Same(t, err, p.mockErr)
	err = ThriftRead(BufferTransport{nil}, &p)
	assert.Same(t, err, p.mockErr)

	// errMethodType
	p1 := TestingWriteReadMethodNotMatch{}
	err = ThriftWrite(BufferTransport{nil}, &p1)
	assert.ErrorIs(t, err, errMethodType)
	err = ThriftRead(BufferTransport{nil}, &p1)
	assert.ErrorIs(t, err, errMethodType)
}
