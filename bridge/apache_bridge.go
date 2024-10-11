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

package bridge

import (
	"fmt"
	"io"
	"reflect"
	"unsafe"

	"github.com/cloudwego/gopkg/bufiox"
	"github.com/cloudwego/gopkg/protocol/thrift"
)

func ApacheReadBridge(iprot interface{}, readFunc func(b []byte) (int, error)) error {
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
			case io.Reader:
				br = bufiox.NewDefaultReader(r)
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

func ApacheWriteBridge(oprot interface{}, bufFunc func() []byte) error {
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
	_, err := bw.WriteBinary(bufFunc())
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
