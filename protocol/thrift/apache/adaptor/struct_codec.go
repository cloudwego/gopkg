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

package adaptor

import (
	"fmt"
	"reflect"

	"github.com/cloudwego/gopkg/protocol/thrift"
)

type fastReader interface {
	FastRead(buf []byte) (int, error)
}

const oldFastWriteMethod = "FastWriteNocopy"

func toFastCodec(p interface{}) (thrift.FastCodec, error) {
	// if struct is from kitex_gen which is generated higher than v0.10.0，just assert gopkg thrift.FastCodec
	if fast, ok := p.(thrift.FastCodec); ok {
		return fast, nil
	}
	// if struct is lower than v0.10.0，the second argument 'bw' from FastWriterNocopy is from kitex package
	// it's not good to import an old kitex dependency, so we have to use reflection
	fast, ok := p.(interface {
		BLength() int
		FastRead(buf []byte) (int, error)
	})
	if !ok {
		return nil, fmt.Errorf("no BLength method for struct")
	}

	method := reflect.ValueOf(p).MethodByName(oldFastWriteMethod)

	if !method.IsValid() {
		return nil, fmt.Errorf("method not found or not exported: %s", oldFastWriteMethod)
	}

	if method.Type().NumIn() != 2 {
		return nil, fmt.Errorf("args num is not ok")
	}

	if method.Type().NumOut() != 1 {
		return nil, fmt.Errorf("resp num is not ok")
	}

	if method.Type().Out(0) != reflect.TypeOf(0) {
		return nil, fmt.Errorf("return type should be int")
	}

	if method.Type().In(0) != reflect.TypeOf([]byte{}) {
		return nil, fmt.Errorf("the first argument should be []byte")
	}

	return &oldFastCodec{
		p:      fast,
		method: method,
	}, nil
}

type oldFastCodec struct {
	p interface {
		BLength() int
		FastRead(buf []byte) (int, error)
	}
	method reflect.Value
}

func (c *oldFastCodec) BLength() int {
	return c.p.BLength()
}

func (c *oldFastCodec) FastWriteNocopy(buf []byte, bw thrift.NocopyWriter) int {
	method := c.method
	out := method.Call([]reflect.Value{reflect.ValueOf(buf), reflect.NewAt(method.Type().In(1), nil)})
	return out[0].Interface().(int)
}

// FastRead actually this function is not used, just to implement the FastCodec interface
func (c *oldFastCodec) FastRead(buf []byte) (int, error) {
	return c.p.FastRead(buf)
}
