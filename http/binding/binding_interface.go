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

// unmarshalParamDecoder handles types that implement UnmarshalParam interface
type unmarshalParamDecoder struct {
	*fieldInfo
}

func newUnmarshalParamDecoder(fi *fieldInfo) fieldDecoder {
	return &unmarshalParamDecoder{fieldInfo: fi}
}

func (d *unmarshalParamDecoder) Decode(ctx *decodeCtx, rv reflect.Value) (bool, error) {
	_, v := d.FetchBindValue(ctx)
	if v == nil {
		return false, nil
	}

	f := d.FieldSetter(rv)
	rv = f.Value()

	var s string
	if ctx.IsStr() {
		s = b2s(v)
	} else {
		s = string(v)
	}

	if err := decodeUnmarshalParamValue(rv, s); err != nil {
		f.Reset()
		return false, err
	}

	return true, nil
}

// textUnmarshalerDecoder handles types that implement encoding.TextUnmarshaler interface
type textUnmarshalerDecoder struct {
	*fieldInfo
}

func newTextUnmarshalerDecoder(fi *fieldInfo) fieldDecoder {
	return &textUnmarshalerDecoder{fieldInfo: fi}
}

func (d *textUnmarshalerDecoder) Decode(ctx *decodeCtx, rv reflect.Value) (bool, error) {
	_, v := d.FetchBindValue(ctx)
	if v == nil {
		return false, nil
	}

	f := d.FieldSetter(rv)
	rv = f.Value()

	var s string
	if ctx.IsStr() {
		s = b2s(v)
	} else {
		s = string(v)
	}

	if err := decodeTextUnmarshalerValue(rv, s); err != nil {
		f.Reset()
		return false, err
	}

	return true, nil
}

func decodeUnmarshalParamValue(rv reflect.Value, s string) error {
	u, ok := rv.Addr().Interface().(paramUnmarshaler)
	if !ok {
		return fmt.Errorf("type does not implement UnmarshalParam")
	}
	if err := u.UnmarshalParam(s); err != nil {
		return fmt.Errorf("UnmarshalParam: %w", err)
	}
	return nil
}

func newUnmarshalParamSliceDecoder(fi *fieldInfo) fieldDecoder {
	ret := newSliceDecoder(fi)
	ret.needcopy = true
	ret.elemDecodeFunc = decodeUnmarshalParamValue
	return ret
}

func decodeTextUnmarshalerValue(rv reflect.Value, s string) error {
	u, ok := rv.Addr().Interface().(textUnmarshaler)
	if !ok {
		return errors.New("type does not implement TextUnmarshaler")
	}
	if err := u.UnmarshalText(s2b(s)); err != nil {
		return fmt.Errorf("UnmarshalText: %w", err)
	}
	return nil
}

func newTextUnmarshalerSliceDecoder(fi *fieldInfo) fieldDecoder {
	ret := newSliceDecoder(fi)
	ret.elemDecodeFunc = decodeTextUnmarshalerValue
	return ret
}
