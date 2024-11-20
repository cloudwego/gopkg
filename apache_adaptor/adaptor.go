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

type ByteBuffer interface {
	// Next reads the next n bytes sequentially and returns the original buffer.
	Next(n int) (p []byte, err error)

	// ReadableLen returns the total length of readable buffer.
	// Return: -1 means unreadable.
	ReadableLen() (n int)
}

type nextReader struct {
	nx ByteBuffer
}

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

func next2Reader(n ByteBuffer) io.Reader {
	return &nextReader{nx: n}
}

func AdaptRead(iprot interface{}, readFunc func(buf []byte) (int, error)) error {
	// 通过过渡手段先让用户的 Apache Codec 变成冷门路径

	// todo
	// 先给 kitex 新版本 TProtocol 补全接口
	// 尝试类型断言（对下一个新版本有效）

	var br bufiox.Reader
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
			// case io.Reader:
			//	br = bufiox.NewDefaultReader(r)
			case ByteBuffer:
				rd := next2Reader(r)
				br = bufiox.NewDefaultReader(rd)
			default:
				return fmt.Errorf("reader not ok")
			}
			break
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

func AdaptWrite(oprot interface{}, writeFunc func() []byte) error {
	var bw bufiox.Writer
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
