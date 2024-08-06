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
	"sync"
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
		return &trans.(*bufferTransport).Buffer
	}
	err := RegisterNewTBinaryProtocol(fn)
	require.NoError(t, err)
	defer func() { newTBinaryProtocol = reflect.Value{} }()

	expectcalls := 0
	for i := 0; i < 2; i++ { // run twice to test cache
		buf := &bytes.Buffer{}
		p0 := &TestingWriteRead{Msg: "hello"}
		err = ThriftWrite(NewBufferTransport(buf), p0) // calls p0.Write
		require.NoError(t, err)
		expectcalls++
		require.Equal(t, expectcalls, called)

		p1 := &TestingWriteRead{}
		err = ThriftRead(NewBufferTransport(buf), p1) // calls p1.Read
		require.NoError(t, err)
		expectcalls++
		require.Equal(t, expectcalls, called)
		require.Equal(t, p0, p1)
	}
}

type TestingWriteReadMethodNotMatch struct{}

func (_ *TestingWriteReadMethodNotMatch) Read(v bool) error  { return nil }
func (_ *TestingWriteReadMethodNotMatch) Write(v bool) error { return nil }

type TestingNoReadWriteMethod struct{}

func (_ *TestingNoReadWriteMethod) Read1(v bool) error  { return nil }
func (_ *TestingNoReadWriteMethod) Write1(v bool) error { return nil }

type TestingWriteReadNotReturningErr struct{}

func (_ *TestingWriteReadNotReturningErr) Read(r io.Reader)  {}
func (_ *TestingWriteReadNotReturningErr) Write(w io.Writer) {}

func TestCheckThriftReadWriteErr(t *testing.T) {
	// reset type cache
	hasThriftRead = sync.Map{}
	hasThriftWrite = sync.Map{}

	var err error

	// errNotPointer
	for i := 0; i < 2; i++ { // run twice to test cache
		err = CheckThriftRead(TestingWriteRead{})
		assert.Same(t, err, errNotPointer)
		err = CheckThriftWrite(TestingWriteRead{})
		assert.Same(t, err, errNotPointer)
	}

	// errNoNewTBinaryProtocol
	for i := 0; i < 2; i++ { // run twice to test cache
		err = CheckThriftRead(&TestingWriteRead{})
		assert.Same(t, err, errNoNewTBinaryProtocol)
		err = CheckThriftWrite(&TestingWriteRead{})
		assert.Same(t, err, errNoNewTBinaryProtocol)
	}

	fn := func(trans TTransport, b0, b1 bool) *bytes.Buffer { return nil }
	RegisterNewTBinaryProtocol(fn)
	defer func() { newTBinaryProtocol = reflect.Value{} }()

	// errMethodType
	for i := 0; i < 2; i++ {
		err = CheckThriftRead(&TestingWriteReadMethodNotMatch{}) // input type err
		assert.ErrorIs(t, err, errMethodType)
		err = CheckThriftWrite(&TestingWriteReadMethodNotMatch{})
		assert.ErrorIs(t, err, errMethodType)
		err = CheckThriftRead(&TestingWriteReadNotReturningErr{}) // return type err
		assert.ErrorIs(t, err, errMethodType)
		err = CheckThriftWrite(&TestingWriteReadNotReturningErr{})
		assert.ErrorIs(t, err, errMethodType)
	}

	// errNoReadMethod, errNoWriteMethod
	for i := 0; i < 2; i++ {
		err = CheckThriftRead(&TestingNoReadWriteMethod{})
		assert.ErrorIs(t, err, errNoReadMethod)
		err = CheckThriftWrite(&TestingNoReadWriteMethod{})
		assert.ErrorIs(t, err, errNoWriteMethod)
	}
}

func TestThriftWriteReadErr(t *testing.T) {
	var err error

	// Read/Write returns err
	p := TestingWriteRead{}
	fn := func(trans TTransport, b0, b1 bool) *bytes.Buffer { return nil }
	RegisterNewTBinaryProtocol(fn)
	defer func() { newTBinaryProtocol = reflect.Value{} }()
	p.mockErr = errors.New("mock")
	err = ThriftWrite(NewBufferTransport(nil), &p)
	assert.Same(t, err, p.mockErr)
	err = ThriftRead(NewBufferTransport(nil), &p)
	assert.Same(t, err, p.mockErr)
}
