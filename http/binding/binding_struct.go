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
	"errors"
	"fmt"
	"reflect"
)

type structDecoder struct {
	dd []fieldDecoder
}

func newStructDecoder(rt reflect.Type, c *DecodeConfig) (Decoder, error) {
	if rt.Kind() != reflect.Struct {
		return nil, errors.New("structDecoder: not struct type")
	}

	ret := &structDecoder{
		dd: make([]fieldDecoder, 0, rt.NumField()),
	}
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if !f.IsExported() && !f.Anonymous {
			// ignore unexported field
			continue
		}
		if f.Anonymous && f.Type.Kind() != reflect.Struct {
			// only anonymous struct is allowed
			continue
		}
		tt := lookupFieldTags(f, c)
		if len(tt) == 0 {
			continue
		}
		fi := newFieldInfo(f, c)
		fi.tagInfos = tt
		dec, err := getFieldDecoder(fi)
		if err != nil {
			return nil, err
		}
		ret.dd = append(ret.dd, dec)
	}

	return ret, nil
}

// Decode implements the Decoder interface
func (d *structDecoder) Decode(req RequestContext, v any) (bool, error) {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer {
		return false, errors.New("not pointer type")
	}
	if rv.IsNil() {
		return false, errors.New("nil pointer")
	}
	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return false, fmt.Errorf("unsupported %s type binding", rv.Type())
	}

	ctx := getDecodeCtx(req)
	defer releaseDecodeCtx(ctx)

	changed := false
	for _, dec := range d.dd {
		fieldChanged, err := dec.Decode(ctx, rv)
		if err != nil {
			return changed, fmt.Errorf("decode field %q err: %w", dec.GetFieldName(), err)
		}
		changed = changed || fieldChanged
	}
	return changed, nil
}
