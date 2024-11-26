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

package apache_adaptor

import (
	"fmt"
	"io"
	"reflect"
	"unsafe"

	"github.com/cloudwego/gopkg/bufiox"
	"github.com/cloudwego/gopkg/protocol/thrift"
)

// AdaptRead receive a kitex binary protocol and read it by given function.
func AdaptRead(p, iprot interface{}) error {
	// for now, we use fastCodec to adapt apache codec.
	// the struct should have the function 'FastRead'
	fastStruct, ok := p.(fastReader)
	if !ok {
		return fmt.Errorf("no codec implementation available")
	}

	var br bufiox.Reader
	// if iprot is from kitex v0.12.0+, use interface assert to get bufiox reader
	if bp, ok := iprot.(bufioxReaderWriter); ok {
		br = bp.GetBufioxReader()
	} else {
		// if iprot is from kitex lower than v0.12.0, use reflection to get reader
		// 	1. in kitex v0.11.0, reader is from the field 'br' which is a bufiox.Reader
		// 		eg: https://github.com/cloudwego/kitex/blob/v0.11.0/pkg/protocol/bthrift/apache/binary_protocol.go#L44
		//  2. in kitex under v0.11.0, reader is from the field 'trans' which is kitex byteBuffer (mostly NetpollByteBuffer)
		// 		eg: https://github.com/cloudwego/kitex/blob/v0.5.2/pkg/remote/codec/thrift/binary_protocol.go#L54
		// in apache thrift v0.13.0, reader is from the field 'trans' which implements the interface io.ReadWriter
		//  eg: https://github.com/apache/thrift/blob/v0.13.0/lib/go/thrift/binary_protocol.go#L33
		fieldNames := []string{"br", "trans"}
		for _, fn := range fieldNames {
			reader, exist, err := getUnexportField(iprot, fn)
			if err != nil {
				return err
			}
			if exist {
				switch r := reader.(type) {
				case bufiox.Reader:
					br = r
				case byteBuffer:
					// if reader is from byteBuffer, Read() function is not always available
					// so use an adaptor to implement Read() by Next() and ReadableLen()
					br = bufiox.NewDefaultReader(byteBuffer2ReadWriter(r))
				case io.ReadWriter:
					// if reader is not byteBuffer but is io.ReadWriter, it suppose to be apache thrift binary protocol
					br = bufiox.NewDefaultReader(r)
				default:
					return fmt.Errorf("reader not ok")
				}
				break
			}
		}
	}
	if br == nil {
		return fmt.Errorf("no available field for reader")
	}

	// read data from iprot
	buf, err := thrift.NewSkipDecoder(br).Next(thrift.STRUCT)
	if err != nil {
		return err
	}

	// unmarshal the data into struct
	_, err = fastStruct.FastRead(buf)
	return err
}

// AdaptWrite receive a kitex binary protocol and write it by given function.
func AdaptWrite(p, oprot interface{}) error {
	// for now, we use fastCodec, the struct should have the function 'FastWrite'
	// but in old kitex_gen, the arguments of FastWrite is not from the same package.
	// so we use reflection to handle this situation.
	fastStruct, err := toFastCodec(p)
	if err != nil {
		return fmt.Errorf("no codec implementation available:%s", err)
	}

	var bw bufiox.Writer
	// if iprot is from kitex v0.12.0+, use interface assert to get bufiox writer
	if bp, ok := oprot.(bufioxReaderWriter); ok {
		bw = bp.GetBufioxWriter()
	} else {
		// if iprot is from kitex lower than v0.12.0, use reflection to get writer
		// 	1. in kitex v0.11.0, writer is from the field 'bw' which is a bufiox.Writer
		// 		eg: https://github.com/cloudwego/kitex/blob/v0.11.0/pkg/protocol/bthrift/apache/binary_protocol.go#L44
		// 	2. in kitex under v0.11.0, writer is from the field 'trans' which is kitex buffer (mostly NetpollByteBuffer)
		// 		eg: https://github.com/cloudwego/kitex/blob/v0.5.2/pkg/remote/codec/thrift/binary_protocol.go#L54
		// in apache thrift v0.13.0, writer is from the field 'trans' which implements the interface io.ReadWriter
		//  eg: https://github.com/apache/thrift/blob/v0.13.0/lib/go/thrift/binary_protocol.go#L33
		fieldNames := []string{"bw", "trans"}
		for _, fn := range fieldNames {
			writer, exist, err := getUnexportField(oprot, fn)
			if err != nil {
				return err
			}
			if exist {
				switch w := writer.(type) {
				case bufiox.Writer:
					bw = w
				case byteBuffer:
					// if writer is from byteBuffer, Write() function is not always available
					// so use an adaptor to implement Write()  by Malloc()
					bw = bufiox.NewDefaultWriter(byteBuffer2ReadWriter(w))
				case io.ReadWriter:
					// if writer is not byteBuffer but is io.ReadWriter, it supposes to be apache thrift binary protocol
					bw = bufiox.NewDefaultWriter(w)
				default:
					return fmt.Errorf("writer type not ok")
				}
				break
			}
		}
	}
	if bw == nil {
		return fmt.Errorf("no available field for writer")
	}

	// use fast codec
	buf := make([]byte, fastStruct.BLength())
	buf = buf[:fastStruct.FastWriteNocopy(buf, nil)]
	_, err = bw.WriteBinary(buf)
	if err != nil {
		return err
	}
	return bw.Flush()
}

// getUnexportField retrieves the value of an unexported struct field.
func getUnexportField(p interface{}, fieldName string) (value interface{}, ok bool, error error) {
	if reflect.TypeOf(p).Kind() != reflect.Ptr {
		return nil, false, fmt.Errorf("%s is not a ptr", p)
	}
	field := reflect.ValueOf(p).Elem().FieldByName(fieldName)
	if field.IsValid() {
		return reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Interface(), true, nil
	}
	return nil, false, nil
}

// bufioxReaderWriter
type bufioxReaderWriter interface {
	GetBufioxReader() bufiox.Reader
	GetBufioxWriter() bufiox.Writer
}
