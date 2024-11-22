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
func AdaptRead(iprot interface{}, readFunc func(buf []byte) (int, error)) error {
	var br bufiox.Reader
	// if iprot is from kitex v0.12.0+, use interface assert to get bufiox reader
	if bp, ok := iprot.(bufioxReaderWriter); ok {
		br = bp.GetBufioxReader()
	} else {
		// if iprot is from kitex version lower than v0.12.0, use reflection to get reader
		// in kitex v0.10.0, reader is from the field 'br' which is a bufiox.Reader
		// in kitex under v0.10.0, reader is from the field 'trans' which is kitex byteBuffer (mostly NetpollByteBuffer)
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
					// so use an adaptor to implement Read()  by Next() and ReadableLen()
					rd := next2Reader(r)
					br = bufiox.NewDefaultReader(rd)
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
	buf, err := thrift.NewSkipDecoder(br).Next(thrift.STRUCT)
	if err != nil {
		return err
	}
	_, err = readFunc(buf)
	return err
}

// AdaptWrite receive a kitex binary protocol and write it by given function.
func AdaptWrite(oprot interface{}, writeFunc func() []byte) error {
	var bw bufiox.Writer
	// if iprot is from kitex v0.12.0+, use interface assert to get bufiox writer
	if bp, ok := oprot.(bufioxReaderWriter); ok {
		bw = bp.GetBufioxWriter()
	} else {
		// if iprot is from kitex version lower than v0.12.0, use reflection to get writer
		// in kitex v0.10.0, writer is from the field 'bw' which is a bufiox.Writer
		// in kitex under v0.10.0, writer is from the field 'trans' which implements the interface io.Writer
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
				case io.Writer:
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
	buf := writeFunc()
	_, err := bw.WriteBinary(buf)
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

// byteBuffer
type byteBuffer interface {
	// Next reads the next n bytes sequentially and returns the original buffer.
	Next(n int) (p []byte, err error)

	// ReadableLen returns the total length of readable buffer.
	// Return: -1 means unreadable.
	ReadableLen() (n int)
}

// nextReader is an adaptor that implement Read() by Next() and ReadableLen()
type nextReader struct {
	nx byteBuffer
}

// Read reads data from the nextReader's internal buffer into p.
func (nr nextReader) Read(p []byte) (n int, err error) {
	readable := nr.nx.ReadableLen()
	if readable == -1 {
		return 0, err
	}
	if readable > len(p) {
		readable = len(p)
	}
	data, err := nr.nx.Next(readable)
	if err != nil {
		return -1, err
	}
	copy(p, data)
	return readable, nil
}

func next2Reader(n byteBuffer) io.Reader {
	return &nextReader{nx: n}
}
