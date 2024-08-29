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
	"errors"
	"testing"

	"github.com/cloudwego/gopkg/bufiox"
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
	bw := bufiox.NewDefaultWriter(buf)

	err = ThriftWrite(bw, v)
	require.Same(t, err, errThriftWriteNotRegistered)

	RegisterThriftWrite(callThriftWrite)
	err = ThriftWrite(bw, v) // calls v.Write
	require.NoError(t, err)
	err = bw.Flush()
	require.NoError(t, err)

	p := &TestingWriteRead{}

	br := bufiox.NewDefaultReader(buf)

	err = ThriftRead(br, p)
	require.Same(t, err, errThriftReadNotRegistered)

	RegisterThriftRead(callThriftRead)
	err = ThriftRead(br, p) // calls p.Read
	require.NoError(t, err)

	require.Equal(t, v.Msg, p.Msg)
}

type TStruct interface { // simulate thrift.TStruct
	Read(r bufiox.Reader) error
	Write(w bufiox.Writer) error
}

type TestingWriteRead struct {
	Msg string
}

func (t *TestingWriteRead) Read(r bufiox.Reader) error {
	b, err := r.Next(5)
	if err != nil {
		return err
	}
	t.Msg = string(b)
	return nil
}

func (t *TestingWriteRead) Write(w bufiox.Writer) error {
	b, err := w.Malloc(5)
	if err != nil {
		return err
	}
	copy(b, t.Msg)
	return nil
}

var errNotThriftTStruct = errors.New("errNotThriftTStruct")

func checkTStruct(v interface{}) error {
	_, ok := v.(TStruct)
	if !ok {
		return errNotThriftTStruct
	}
	return nil
}

func callThriftRead(rw bufiox.Reader, v interface{}) error {
	p, ok := v.(TStruct)
	if !ok {
		return errNotThriftTStruct
	}
	return p.Read(rw)
}

func callThriftWrite(rw bufiox.Writer, v interface{}) error {
	p, ok := v.(TStruct)
	if !ok {
		return errNotThriftTStruct
	}
	return p.Write(rw)
}
