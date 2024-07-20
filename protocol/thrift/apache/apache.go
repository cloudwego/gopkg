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

// Package apache contains code for working with apache thrift indirectly
//
// It acts as a bridge between generated code which relies on apache codec like:
//
//	Write(p thrift.TProtocol) error
//	Read(p thrift.TProtocol) error
//
// and kitex ecosystem.
//
// Because we're deprecating apache thrift, all kitex ecosystem code will not rely on apache thrift
// except one pkg: `github.com/cloudwego/kitex/pkg/protocol/bthrift`. Why is the package chosen?
// All legacy generated code relies on it, and we may not be able to update the code in a brief timeframe.
// So the package is chosen to register `thrift.NewTBinaryProtocol` to this package in order to use it
// without importing `github.com/apache/thrift`
//
// ThriftRead or ThriftWrite is implemented for calling Read/Write
// without knowing the interface of `thrift.TProtocol`.
// Since we already have `thrift.NewTBinaryProtocol`, we only need to check:
// if the return value of `thrift.NewTBinaryProtocol` implements
// the input which is `thrift.TProtocol` of Read/Write
//
// For new generated code,
// it no longer uses the `github.com/cloudwego/kitex/pkg/protocol/bthrift`
package apache

import (
	"errors"
	"fmt"
	"reflect"
)

var (
	newTBinaryProtocol reflect.Value

	rvTrue = reflect.ValueOf(true) // for calling NewTBinaryProtocol
)

var (
	ttransportType = reflect.TypeOf((*TTransport)(nil)).Elem()
	errorType      = reflect.TypeOf((*error)(nil)).Elem()
)

var (
	errNoNewTBinaryProtocol = errors.New("thrift.NewTBinaryProtocol method not registered. Make sure you're using apache/thrift == 0.13.0 and clouwdwego/kitex >= 0.11.0")
	errNotPointer           = errors.New("input not pointer")
	errNoReadMethod         = errors.New("thrift.TStruct `Read` method not found")
	errNoWriteMethod        = errors.New("thrift.TStruct `Write` method not found")

	errMethodType  = errors.New("method type not match")
	errNewFuncType = errors.New("function type not match")
)

func errNewFuncTypeNotMatch(t reflect.Type) error {
	const expect = "func(thrift.TTransport, bool, bool) *thrift.TBinaryProtocol"
	return fmt.Errorf("%w:\n\texpect: %s\n\t   got: %s", errNewFuncType, expect, t)
}

func errReadWriteMethodNotMatch(t reflect.Type) error {
	const expect = "func(thrift.TProtocol) error"
	return fmt.Errorf("%w:\n\texpect: %s\n\t   got: %s", errMethodType, expect, t)
}

// RegisterNewTBinaryProtocol accepts `thrift.NewTBinaryProtocol` func and save it for later use.
func RegisterNewTBinaryProtocol(fn interface{}) error {
	v := reflect.ValueOf(fn)
	t := v.Type()

	// check it's func
	if t.Kind() != reflect.Func {
		return errNewFuncTypeNotMatch(t)
	}

	// check "func(thrift.TTransport, bool, bool) *thrift.TBinaryProtocol"
	// can also check with t.String() instead of field by field?
	if t.NumIn() != 3 ||
		!t.In(0).Implements(ttransportType) ||
		t.In(1).Kind() != reflect.Bool ||
		t.In(2).Kind() != reflect.Bool {
		return errNewFuncTypeNotMatch(t)
	}
	if t.NumOut() != 1 {
		// not checking if it's thrift.TProtocol
		// but in ThriftRead/ThriftWrite, we will check if it implements the input of Read/Write
		// so we can make it easier to test.
		return errNewFuncTypeNotMatch(t)
	}
	newTBinaryProtocol = v
	return nil
}

func checkThriftReadWriteFuncType(t reflect.Type) error {
	if !newTBinaryProtocol.IsValid() {
		return errNoNewTBinaryProtocol
	}

	// checks `func(thrift.TProtocol) error`
	if t.NumIn() != 1 || t.In(0).Kind() != reflect.Interface ||
		!newTBinaryProtocol.Type().Out(0).Implements(t.In(0)) {
		return errReadWriteMethodNotMatch(t)
	}
	if t.NumOut() != 1 ||
		!t.Out(0).Implements(errorType) {
		return errReadWriteMethodNotMatch(t)
	}
	return nil
}

// ThriftRead calls Read method of v.
//
// RegisterNewTBinaryProtocol must be called with `thrift.NewTBinaryProtocol`
// before using this func.
func ThriftRead(t TTransport, v interface{}) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr {
		// Read/Write method is always pointer receiver
		return errNotPointer
	}
	rfunc := rv.MethodByName("Read")

	// check Read func signature: func(thrift.TProtocol) error
	if !rfunc.IsValid() || rfunc.Kind() != reflect.Func {
		return errNoReadMethod
	}
	if err := checkThriftReadWriteFuncType(rfunc.Type()); err != nil {
		return err
	}

	// iprot := NewTBinaryProtocol(t, true, true)
	iprot := newTBinaryProtocol.Call([]reflect.Value{reflect.ValueOf(t), rvTrue, rvTrue})[0]

	// err := v.Read(iprot)
	err := rfunc.Call([]reflect.Value{iprot})[0]
	if err.IsNil() {
		return nil
	}
	return err.Interface().(error)
}

// ThriftWrite calls Write method of v.
//
// RegisterNewTBinaryProtocol must be called with `thrift.NewTBinaryProtocol`
// before using this func.
func ThriftWrite(t TTransport, v interface{}) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr {
		// Read/Write method is always pointer receiver
		return errNotPointer
	}
	wfunc := rv.MethodByName("Write")

	// check Write func signature: func(thrift.TProtocol) error
	if !wfunc.IsValid() || wfunc.Kind() != reflect.Func {
		return errNoWriteMethod
	}
	if err := checkThriftReadWriteFuncType(wfunc.Type()); err != nil {
		return err
	}

	// oprot := NewTBinaryProtocol(t, true, true)
	oprot := newTBinaryProtocol.Call([]reflect.Value{reflect.ValueOf(t), rvTrue, rvTrue})[0]

	// err := v.Write(oprot)
	err := wfunc.Call([]reflect.Value{oprot})[0]
	if err.IsNil() {
		return nil
	}
	return err.Interface().(error)
}
