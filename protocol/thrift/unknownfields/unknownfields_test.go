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

package unknownfields

import (
	"testing"

	"github.com/cloudwego/gopkg/protocol/thrift"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnknownFields(t *testing.T) {
	type A struct {
		_unknownFields []byte
	}
	a := &A{}

	// prepare data
	b := make([]byte, 0, 1024)
	expect := make([]UnknownField, 0, 11) // 11 testcases

	// BOOL, fid=1
	b = thrift.Binary.AppendFieldBegin(b, thrift.BOOL, 1)
	b = thrift.Binary.AppendBool(b, true)
	expect = append(expect, UnknownField{ID: 1, Type: thrift.BOOL, Value: true})

	// BYTE, fid=2
	b = thrift.Binary.AppendFieldBegin(b, thrift.BYTE, 2)
	b = thrift.Binary.AppendByte(b, 2)
	expect = append(expect, UnknownField{ID: 2, Type: thrift.BYTE, Value: int8(2)})

	// I16, fid=3
	b = thrift.Binary.AppendFieldBegin(b, thrift.I16, 3)
	b = thrift.Binary.AppendI16(b, 3)
	expect = append(expect, UnknownField{ID: 3, Type: thrift.I16, Value: int16(3)})

	// I32, fid=4
	b = thrift.Binary.AppendFieldBegin(b, thrift.I32, 4)
	b = thrift.Binary.AppendI32(b, 4)
	expect = append(expect, UnknownField{ID: 4, Type: thrift.I32, Value: int32(4)})

	// I64, fid=5
	b = thrift.Binary.AppendFieldBegin(b, thrift.I64, 5)
	b = thrift.Binary.AppendI64(b, 5)
	expect = append(expect, UnknownField{ID: 5, Type: thrift.I64, Value: int64(5)})

	// DOUBLE, fid=6
	b = thrift.Binary.AppendFieldBegin(b, thrift.DOUBLE, 6)
	b = thrift.Binary.AppendDouble(b, 6)
	expect = append(expect, UnknownField{ID: 6, Type: thrift.DOUBLE, Value: float64(6)})

	// STRING, fid=7
	b = thrift.Binary.AppendFieldBegin(b, thrift.STRING, 7)
	b = thrift.Binary.AppendString(b, "7")
	expect = append(expect, UnknownField{ID: 7, Type: thrift.STRING, Value: "7"})

	// MAP, fid=8
	b = thrift.Binary.AppendFieldBegin(b, thrift.MAP, 8)
	b = thrift.Binary.AppendMapBegin(b, thrift.DOUBLE, thrift.DOUBLE, 1)
	b = thrift.Binary.AppendDouble(b, 8.1)
	b = thrift.Binary.AppendDouble(b, 8.2)
	expect = append(expect, UnknownField{
		ID: 8, Type: thrift.MAP,
		KeyType: thrift.DOUBLE, ValType: thrift.DOUBLE,
		Value: []UnknownField{
			{Type: thrift.DOUBLE, Value: float64(8.1)},
			{Type: thrift.DOUBLE, Value: float64(8.2)},
		},
	})

	// SET, fid=9
	b = thrift.Binary.AppendFieldBegin(b, thrift.SET, 9)
	b = thrift.Binary.AppendSetBegin(b, thrift.I64, 1)
	b = thrift.Binary.AppendI64(b, 9)
	expect = append(expect, UnknownField{
		ID: 9, Type: thrift.SET,
		ValType: thrift.I64,
		Value: []UnknownField{
			{Type: thrift.I64, Value: int64(9)},
		},
	})

	// LIST, fid=10
	b = thrift.Binary.AppendFieldBegin(b, thrift.LIST, 10)
	b = thrift.Binary.AppendListBegin(b, thrift.I64, 1)
	b = thrift.Binary.AppendI64(b, 10)
	expect = append(expect, UnknownField{
		ID: 10, Type: thrift.LIST,
		ValType: thrift.I64,
		Value: []UnknownField{
			{Type: thrift.I64, Value: int64(10)},
		},
	})

	// STRUCT with 1 field I64, fid=11,1
	b = thrift.Binary.AppendFieldBegin(b, thrift.STRUCT, 11)
	b = thrift.Binary.AppendFieldBegin(b, thrift.I64, 1)
	b = thrift.Binary.AppendI64(b, 11)
	b = thrift.Binary.AppendFieldStop(b)
	expect = append(expect, UnknownField{ID: 11, Type: thrift.STRUCT, Value: []UnknownField{
		{ID: 1, Type: thrift.I64, Value: int64(11)},
	}})

	// decode
	a._unknownFields = b
	fields, err := GetUnknownFields(a)
	require.NoError(t, err)
	require.Equal(t, len(expect), len(fields))

	// test fields
	for i := range fields {
		assert.Equal(t, expect[i], fields[i])
	}
}
