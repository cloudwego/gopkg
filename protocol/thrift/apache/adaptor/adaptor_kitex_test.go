// Copyright 2024 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package adaptor_test

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/cloudwego/gopkg/protocol/thrift/apache/adaptor"
	"github.com/stretchr/testify/require"

	"github.com/cloudwego/gopkg/bufiox"
	"github.com/cloudwego/gopkg/protocol/thrift"
)

type kitexGen int

const (
	// newKitexGen means the kitex struct generate by tool v0.11.0+, the second argument of FastWriteNoCopy is a NocopyWriter interface.
	newKitexGen kitexGen = iota
	// oldKitexGen means the kitex struct generate before tool v0.11.0, the second argument of FastWriteNoCopy is a Kitex NocopyWriter type.
	oldKitexGen
)

// TestKitexBinaryProtocol
func TestKitexBinaryProtocol(t *testing.T) {
	// In different versions of kitex, there are three scenarios for the implementation of the binary protocol:
	// In versions before v0.11.0, the binary protocol processes data using remote.ByteBuffer.
	// In versions after v0.11.0, the binary protocol processes data using bufiox.
	// In versions after v0.13.0 (to be released), the binary protocol is similar to v0.11.0, also processing data using bufiox, but it provides the GetBufiox interface for easier access to bufiox.
	// We will test these three scenarios separately.

	// kitex before v0.11.0 binary protocol with kitex struct before v0.11.0
	testAdaptor(t, oldKitexGen, mockBinaryProtocolV0100())

	// kitex v0.11.0 binary protocol with kitex struct after v0.11.0
	testAdaptor(t, oldKitexGen, mockBinaryProtocolV0110())

	// kitex v0.13.0 binary protocol with kitex struct after v0.11.0
	testAdaptor(t, oldKitexGen, mockBinaryProtocolV0130())

	// kitex before v0.11.0 binary protocol with kitex struct after v0.11.0
	testAdaptor(t, newKitexGen, mockBinaryProtocolV0100())

	// kitex v0.11.0 binary protocol with kitex struct after v0.11.0
	testAdaptor(t, newKitexGen, mockBinaryProtocolV0110())

	// kitex v0.13.0 binary protocol with kitex struct after v0.11.0
	testAdaptor(t, newKitexGen, mockBinaryProtocolV0130())
}

// testAdaptor is used to simulate the process of the Apache adaptor obtaining a struct and a binary protocol, bridging the data serialization process.
// In this test, first create a specific struct, test converting it into a binary stream using the adaptor.
// Then deserialize it using the adaptor into a new struct, and compare the contents of the new and old structs to verify the correctness of the Apache adaptor.
func testAdaptor(t *testing.T, kitexStruct kitexGen, bp interface{}) {
	var from, to interface{}
	switch kitexStruct {
	case oldKitexGen:
		from = mockOldKitexStruct()
		to = &oldKitexStruct{}
	case newKitexGen:
		from = mockNewKitexStruct()
		to = &newKitexStruct{}
	default:
		require.Error(t, fmt.Errorf("kitex gen type not ok"))
	}
	err := adaptor.AdaptWrite(from, bp)
	require.NoError(t, err)
	err = adaptor.AdaptRead(to, bp)
	require.NoError(t, err)
	require.True(t, reflect.DeepEqual(from, to))
}

// binaryProtocolV0100 mocks the kitex thrift binary protocol struct before v0.11.0 (v0.11.0 is not included), with remote.ByteBuffer as the field 'trans' to handle the data.
// https://github.com/cloudwego/kitex/blob/v0.5.2/pkg/protocol/bthrift/apache/binary_protocol.go#L44
type binaryProtocolV0100 struct {
	trans mockRemoteByteBuffer
}

// mockRemoteByteBuffer mocks the kitex remote.ByteBuffer, which is the core abstraction of buffer in Kitex.
// https://github.com/cloudwego/kitex/blob/v0.5.2/pkg/remote/bytebuf.go#L46
type mockRemoteByteBuffer interface {
	// Next reads the next n bytes sequentially and returns the original buffer.
	Next(n int) (p []byte, err error)
	// ReadableLen returns the total length of readable buffer.
	// Return: -1 means unreadable.
	ReadableLen() (n int)
	// Malloc n bytes sequentially in the writer buffer.
	Malloc(n int) (buf []byte, err error)
	// another function is not used in apache adaptor
}

func mockBinaryProtocolV0100() *binaryProtocolV0100 {
	return &binaryProtocolV0100{trans: &simpleBuffer{
		data: make([]byte, 100),
	}}
}

// binaryProtocolV0110 mocks the kitex thrift binary protocol struct after v0.11.0, with bufiox reader and writer to handle the data.
// https://github.com/cloudwego/kitex/blob/v0.11.0/pkg/protocol/bthrift/apache/binary_protocol.go#L44
type binaryProtocolV0110 struct {
	r *thrift.BufferReader
	w *thrift.BufferWriter

	br bufiox.Reader
	bw bufiox.Writer
}

func mockBinaryProtocolV0110() *binaryProtocolV0110 {
	buffer := bytes.NewBuffer(nil)
	br := bufiox.NewDefaultReader(buffer)
	bw := bufiox.NewDefaultWriter(buffer)
	return &binaryProtocolV0110{
		r:  thrift.NewBufferReader(br),
		w:  thrift.NewBufferWriter(bw),
		br: br,
		bw: bw,
	}
}

// binaryProtocolV0130 mocks the kitex thrift binary protocol struct after v0.13.0 (currently unreleased)
// It's almost the same with binaryProtocolV0110, but have two more function 'GetBufioxReader' and 'GetBufioxWriter', in order to get the bufiox more convenient without reflection.
// https://github.com/cloudwego/kitex/blob/v0.13.0/pkg/protocol/bthrift/apache/binary_protocol.go#L44
type binaryProtocolV0130 struct {
	r *thrift.BufferReader
	w *thrift.BufferWriter

	br bufiox.Reader
	bw bufiox.Writer
}

func mockBinaryProtocolV0130() *binaryProtocolV0130 {
	buffer := bytes.NewBuffer(nil)
	br := bufiox.NewDefaultReader(buffer)
	bw := bufiox.NewDefaultWriter(buffer)
	return &binaryProtocolV0130{
		r:  thrift.NewBufferReader(br),
		w:  thrift.NewBufferWriter(bw),
		br: br,
		bw: bw,
	}
}

func (bp *binaryProtocolV0130) GetBufioxReader() bufiox.Reader {
	return bp.br
}

func (bp *binaryProtocolV0130) GetBufioxWriter() bufiox.Writer {
	return bp.bw
}

// simpleBuffer is a very simple implementation of mockRemoteByteBuffer
type simpleBuffer struct {
	data []byte
	rc   int
	wc   int
}

func (bb *simpleBuffer) Next(n int) ([]byte, error) {
	if bb.rc+n > len(bb.data) {
		return nil, errors.New("not enough data to read")
	}
	p := bb.data[bb.rc : bb.rc+n]
	bb.rc += n
	return p, nil
}

func (bb *simpleBuffer) ReadableLen() int {
	return len(bb.data) - bb.rc
}

func (bb *simpleBuffer) Malloc(n int) ([]byte, error) {
	if bb.wc+n > len(bb.data) {
		return nil, errors.New("not enough space to allocate")
	}
	buf := bb.data[bb.wc : bb.wc+n]
	bb.wc += n
	return buf, nil
}

// oldKitexStruct mocks the kitex struct generate before tool v0.11.0, the second argument of FastWriteNoCopy is a Kitex NocopyWriter type.
type oldKitexStruct struct {
	FBool bool  `thrift:"FBool,1,required" frugal:"1,required,bool" json:"FBool"`
	FByte int8  `thrift:"FByte,2" frugal:"2,default,byte" json:"FByte"`
	I8    int8  `thrift:"I8,3" frugal:"3,default,i8" json:"I8"`
	I16   int16 `thrift:"I16,4" frugal:"4,default,i16" json:"I16"`
}

func (p *oldKitexStruct) BLength() int {
	return len(mockBinary)
}

func (p *oldKitexStruct) FastRead(buf []byte) (int, error) {
	if bytes.Equal(buf, mockBinary) {
		p.FBool = true
		p.FByte = 3
		p.I8 = 1
		p.I16 = 2
		return len(buf), nil
	}
	return -1, fmt.Errorf("data error")
}

// binaryWriter does not implement nocopy writer
func (p *oldKitexStruct) FastWriteNocopy(buf []byte, binaryWriter interface{}) int {
	if reflect.DeepEqual(p, mockOldKitexStruct()) {
		copy(buf, mockBinary)
		return len(buf)
	}
	return -1
}

var mockBinary = []byte{2, 0, 1, 1, 3, 0, 2, 3, 3, 0, 3, 1, 6, 0, 4, 0, 2, 0}

func mockOldKitexStruct() *oldKitexStruct {
	return &oldKitexStruct{
		FBool: true,
		FByte: 3,
		I8:    1,
		I16:   2,
	}
}

// newKitexStruct mocks the kitex struct generate by tool v0.11.0+, the second argument of FastWriteNoCopy is a NocopyWriter interface.
type newKitexStruct struct {
	FBool bool  `thrift:"FBool,1,required" frugal:"1,required,bool" json:"FBool"`
	FByte int8  `thrift:"FByte,2" frugal:"2,default,byte" json:"FByte"`
	I8    int8  `thrift:"I8,3" frugal:"3,default,i8" json:"I8"`
	I16   int16 `thrift:"I16,4" frugal:"4,default,i16" json:"I16"`
}

func (p *newKitexStruct) BLength() int {
	return len(mockBinary)
}

func (p *newKitexStruct) FastRead(buf []byte) (int, error) {
	if bytes.Equal(buf, mockBinary) {
		p.FBool = true
		p.FByte = 3
		p.I8 = 1
		p.I16 = 2
		return len(buf), nil
	}
	return -1, fmt.Errorf("data error")
}

func (p *newKitexStruct) FastWriteNocopy(buf []byte, binaryWriter thrift.NocopyWriter) int {
	if reflect.DeepEqual(p, mockNewKitexStruct()) {
		copy(buf, mockBinary)
		return len(buf)
	}
	return -1
}

func mockNewKitexStruct() *newKitexStruct {
	return &newKitexStruct{
		FBool: true,
		FByte: 3,
		I8:    1,
		I16:   2,
	}
}
