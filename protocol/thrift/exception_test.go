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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplicationException(t *testing.T) {
	ex1 := NewApplicationException(1, "t1")
	b := make([]byte, ex1.BLength())
	n := ex1.FastWriteNocopy(b, nil)
	assert.Equal(t, len(b), n)

	ex2 := NewApplicationException(0, "")
	n, err := ex2.FastRead(b)
	require.NoError(t, err)
	assert.Equal(t, len(b), n)
	assert.Equal(t, int32(1), ex2.TypeID())
	assert.Equal(t, int32(1), ex2.TypeId())
	assert.Equal(t, "t1", ex2.Msg())

	ex3 := NewApplicationException(1, "")
	assert.Equal(t, defaultApplicationExceptionMessage[ex3.TypeID()], ex3.Error())

	ex4 := NewApplicationException(999, "")
	assert.Equal(t, "unknown exception type [999]", ex4.Error())

	t.Log(ex4.String()) // ...
}

type testTException struct{}

func (testTException) Error() string { return "testTException" }
func (testTException) TypeId() int32 { return -1 }

func TestPrependError(t *testing.T) {
	var ok bool

	// case TransportException
	ex0 := NewTransportException(1, "world")
	err0 := PrependError("hello ", ex0)
	ex0, ok = err0.(*TransportException)
	require.True(t, ok)
	assert.Equal(t, int32(1), ex0.TypeID())
	assert.Equal(t, "hello world", ex0.Error())

	// case ProtocolException
	ex1 := NewProtocolException(2, "world")
	err1 := PrependError("hello ", ex1)
	ex1, ok = err1.(*ProtocolException)
	require.True(t, ok)
	assert.Equal(t, int32(2), ex1.TypeID())
	assert.Equal(t, "hello world", ex1.Error())

	// case ApplicationException
	ex2 := NewApplicationException(3, "world")
	err2 := PrependError("hello ", ex2)
	ex2, ok = err2.(*ApplicationException)
	require.True(t, ok)
	assert.Equal(t, int32(3), ex2.TypeID())
	assert.Equal(t, "hello world", ex2.Error())

	// case tException
	ex3 := testTException{}
	err3 := PrependError("hello ", ex3)
	ex4, ok := err3.(*ApplicationException)
	require.True(t, ok)
	assert.Equal(t, int32(-1), ex4.TypeID())
	assert.Equal(t, "hello testTException", ex4.Error())

	// case normal error
	err4 := PrependError("hello ", errors.New("world"))
	_, ok = err4.(tException)
	require.False(t, ok)
	assert.Equal(t, "hello world", err4.Error())
}
