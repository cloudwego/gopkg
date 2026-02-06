/*
 * Copyright 2025 CloudWeGo Authors
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

package binding

import (
	"reflect"
)

type fieldInfo struct {
	index     int
	fieldName string
	fieldKind reflect.Kind
	fieldType reflect.Type

	tagInfos []*tagInfo // ordered by precedence

	// Cached JSON unmarshal function from config
	jsonUnmarshal func(data []byte, v interface{}) error
}

func newFieldInfo(f reflect.StructField, c *DecodeConfig) *fieldInfo {
	rt := dereferenceType(f.Type)
	fi := &fieldInfo{
		index:         f.Index[len(f.Index)-1],
		fieldName:     f.Name,
		fieldKind:     rt.Kind(),
		fieldType:     rt,
		jsonUnmarshal: c.getJSONUnmarshal(),
	}
	return fi
}

func (f *fieldInfo) FieldSetter(rv reflect.Value) fieldSetter {
	return newFieldSetter(f, rv)
}

// FieldValue is shortcut of FieldSetter(rv).Value()
func (f *fieldInfo) FieldValue(rv reflect.Value) reflect.Value {
	rv = dereference2lvalue(rv)
	if f.index >= 0 {
		rv = rv.Field(f.index)
	}
	return dereference2lvalue(rv)
}

func (f *fieldInfo) FetchBindValue(ctx *decodeCtx) (tag string, v []byte) {
	ctx.Reset()
	for _, ti := range f.tagInfos {
		if ti.Getter == nil {
			continue
		}
		ti.Getter(ctx.RequestContext, ti.Name, &ctx.GetResult)
		if v := ctx.GetResult.Value(); v != nil {
			return ti.Tag, v
		}
	}
	return "", nil
}

// GetFieldName returns the field name for error reporting
func (f *fieldInfo) GetFieldName() string {
	return f.fieldName
}
