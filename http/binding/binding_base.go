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
	"fmt"
	"reflect"
	"strconv"
)

type baseDecoder struct {
	*fieldInfo

	// needcopy indicates if string values need to be copied.
	// For string target fields, we may need to copy the byte slice to create a proper string.
	// When data comes from string sources (headers, query params), we can use b2s() for zero-copy.
	// When data comes from other sources, string(v) creates a copy which is safe for string fields.
	needcopy    bool
	decodeValue func(rv reflect.Value, s string) error
}

func newBaseDecoder(fi *fieldInfo) fieldDecoder {
	dec := &baseDecoder{fieldInfo: fi}
	fn := getBaseDecodeByKind(fi.fieldKind)
	if fn == nil {
		// Use method that has access to jsonUnmarshal
		fn = dec.decodeJSONValue
	}
	// Set needcopy flag for string fields that may require copying
	dec.needcopy = fi.fieldKind == reflect.String
	dec.decodeValue = fn
	return dec
}

func (d *baseDecoder) Decode(ctx *decodeCtx, rv reflect.Value) (bool, error) {
	_, v := d.FetchBindValue(ctx)
	if v == nil {
		return false, nil
	}

	f := d.FieldSetter(rv)
	rv = f.Value()

	// Optimize string conversion based on data source:
	// - If data comes from string sources (headers, query params), use b2s() for zero-copy conversion
	// - If decoding to string field from non-string source, use string(v) to create a safe copy
	s := b2s(v)
	if d.needcopy && !ctx.IsStr() {
		s = string(v)
	}
	if err := d.decodeValue(rv, s); err != nil {
		f.Reset()
		return false, fmt.Errorf("unable to decode '%s' as %s: %w", s, d.fieldType.String(), err)
	}
	return true, nil
}

// use slice for better performance,
var type2decoder = [...]func(rv reflect.Value, s string) error{
	reflect.Bool:    decodeBool,
	reflect.Uint:    decodeUint,
	reflect.Uint8:   decodeUint8,
	reflect.Uint16:  decodeUint16,
	reflect.Uint32:  decodeUint32,
	reflect.Uint64:  decodeUint64,
	reflect.Int:     decodeInt,
	reflect.Int8:    decodeInt8,
	reflect.Int16:   decodeInt16,
	reflect.Int32:   decodeInt32,
	reflect.Int64:   decodeInt64,
	reflect.String:  decodeString,
	reflect.Float32: decodeFloat32,
	reflect.Float64: decodeFloat64,
}

func getBaseDecodeByKind(k reflect.Kind) (ret func(rv reflect.Value, s string) error) {
	if int(k) >= len(type2decoder) {
		return nil
	}
	return type2decoder[k]
}

// decodeJSONValue is a method on baseDecoder that uses the configured JSON unmarshal function
func (d *baseDecoder) decodeJSONValue(rv reflect.Value, s string) error {
	return d.jsonUnmarshal(s2b(s), rv.Addr().Interface())
}

func decodeBool(rv reflect.Value, s string) error {
	val, err := strconv.ParseBool(s)
	if err == nil {
		*(*bool)(rvUnsafePointer(&rv)) = val
	}
	return err

}

func decodeUint(rv reflect.Value, s string) error {
	val, err := strconv.ParseUint(s, 10, 0)
	if err == nil {
		*(*uint)(rvUnsafePointer(&rv)) = uint(val)
	}
	return err

}

func decodeUint8(rv reflect.Value, s string) error {
	val, err := strconv.ParseUint(s, 10, 8)
	if err == nil {
		*(*uint8)(rvUnsafePointer(&rv)) = uint8(val)
	}
	return err
}

func decodeUint16(rv reflect.Value, s string) error {
	val, err := strconv.ParseUint(s, 10, 16)
	if err == nil {
		*(*uint16)(rvUnsafePointer(&rv)) = uint16(val)
	}
	return err
}

func decodeUint32(rv reflect.Value, s string) error {
	val, err := strconv.ParseUint(s, 10, 32)
	if err == nil {
		*(*uint32)(rvUnsafePointer(&rv)) = uint32(val)
	}
	return err

}

func decodeUint64(rv reflect.Value, s string) error {
	val, err := strconv.ParseUint(s, 10, 64)
	if err == nil {
		*(*uint64)(rvUnsafePointer(&rv)) = val
	}
	return err

}

func decodeInt(rv reflect.Value, s string) error {
	val, err := strconv.Atoi(s)
	if err == nil {
		*(*int)(rvUnsafePointer(&rv)) = val
	}
	return err

}

func decodeInt8(rv reflect.Value, s string) error {
	val, err := strconv.ParseInt(s, 10, 8)
	if err == nil {
		*(*int8)(rvUnsafePointer(&rv)) = int8(val)
	}
	return err

}

func decodeInt16(rv reflect.Value, s string) error {
	val, err := strconv.ParseInt(s, 10, 16)
	if err == nil {
		*(*int16)(rvUnsafePointer(&rv)) = int16(val)
	}
	return err
}

func decodeInt32(rv reflect.Value, s string) error {
	val, err := strconv.ParseInt(s, 10, 32)
	if err == nil {
		*(*int32)(rvUnsafePointer(&rv)) = int32(val)
	}
	return err
}

func decodeInt64(rv reflect.Value, s string) error {
	val, err := strconv.ParseInt(s, 10, 64)
	if err == nil {
		*(*int64)(rvUnsafePointer(&rv)) = val
	}
	return err
}

func decodeString(rv reflect.Value, s string) error {
	*(*string)(rvUnsafePointer(&rv)) = s
	return nil
}

func decodeFloat32(rv reflect.Value, s string) error {
	val, err := strconv.ParseFloat(s, 32)
	if err == nil {
		*(*float32)(rvUnsafePointer(&rv)) = float32(val)
	}
	return err
}

func decodeFloat64(rv reflect.Value, s string) error {
	val, err := strconv.ParseFloat(s, 64)
	if err == nil {
		*(*float64)(rvUnsafePointer(&rv)) = val
	}
	return err
}
