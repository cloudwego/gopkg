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
	"io"
	"testing"
)

// TestApacheProtocol
func TestApacheProtocol(t *testing.T) {
	// Apache Protocol implement the interface TProtocol (https://github.com/apache/thrift/blob/v0.13.0/lib/go/thrift/protocol.go#L33)
	// The implementation classes of this interface include many protocols.
	// Such as TBinaryProtocol, TCompactProtocol, THeaderProtocol, and TJsonProtocol.
	// However, for users of cloudwego/kitex, only TBinaryProtocol and TCompactProtocol are used
	// So we only support to adapt these two protocols.

	// apache binary protocol with old kitex struct
	testAdaptor(t, oldKitexGen, mockTBinaryProtocol())

	// apache binary protocol with new kitex struct
	testAdaptor(t, newKitexGen, mockTBinaryProtocol())

	// apache compact protocol with old kitex struct
	testAdaptor(t, oldKitexGen, mockTCompactProtocol())

	// apache compact protocol with new kitex struct
	testAdaptor(t, newKitexGen, mockTCompactProtocol())
}

// tBinaryProtocol
// https://github.com/apache/thrift/blob/v0.13.0/lib/go/thrift/binary_protocol.go#L33
type tBinaryProtocol struct {
	trans tRichTransport
}

func mockTBinaryProtocol() *tBinaryProtocol {
	return &tBinaryProtocol{
		trans: bytes.NewBuffer(nil),
	}
}

// tCompactProtocol
// https://github.com/apache/thrift/blob/v0.13.0/lib/go/thrift/compact_protocol.go#L88
type tCompactProtocol struct {
	trans tRichTransport
}

func mockTCompactProtocol() *tCompactProtocol {
	return &tCompactProtocol{
		trans: bytes.NewBuffer(nil),
	}
}

// https://github.com/apache/thrift/blob/v0.13.0/lib/go/thrift/trans.go
type tRichTransport interface {
	io.ReadWriter
	// another interfaces are not used in apache adaptor, just ignore them.
}
