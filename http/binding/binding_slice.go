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
	"unsafe"
)

type decodeSliceFunc func(d *sliceDecoder, p unsafe.Pointer, vv [][]byte) error

type sliceDecoder struct {
	*fieldInfo

	// needcopy indicates if string elements need to be copied.
	// For slice/array of strings, we need to decide whether to use b2s() (zero-copy from string sources)
	// or string(v) (safe copy from non-string sources).
	needcopy bool
	arrayLen int // -1 if not array; used to validate array length matches exactly

	decodeFunc     decodeSliceFunc
	elemDecodeFunc func(rv reflect.Value, s string) error
}

func newSliceDecoder(fi *fieldInfo) *sliceDecoder {
	arrayLen := -1
	if fi.fieldKind == reflect.Array {
		arrayLen = fi.fieldType.Len()
	}
	elemKind := fi.fieldType.Elem().Kind()
	return &sliceDecoder{
		fieldInfo: fi,
		// Set needcopy for string elements to optimize conversion
		needcopy:       elemKind == reflect.String,
		arrayLen:       arrayLen,
		decodeFunc:     type2DecodeSliceFunc[fi.fieldType],
		elemDecodeFunc: getBaseDecodeByKind(elemKind),
	}
}

func (d *sliceDecoder) Decode(ctx *decodeCtx, rv reflect.Value) (bool, error) {
	if d.elemDecodeFunc == nil {
		return false, fmt.Errorf("unsupported slice element type: %s", d.fieldType)
	}

	ctx.Reset()

	var bs [][]byte
	for _, ti := range d.tagInfos {
		if ti.Getter == nil {
			continue
		}
		ti.Getter(ctx.RequestContext, ti.Name, &ctx.GetResult)
		if bs = ctx.GetResult.Values(); len(bs) > 0 {
			break
		}
	}

	if len(bs) == 0 {
		return false, nil
	}

	// Arrays require exact length match (unlike slices which can be resized)
	if d.arrayLen > 0 && d.arrayLen != len(bs) {
		return false, fmt.Errorf("%q is not valid value for %s", bs, d.fieldType)
	}

	f := d.FieldSetter(rv)
	fv := f.Value()

	// Use optimized type-specific decoder if available
	if d.arrayLen < 0 && d.decodeFunc != nil {
		if err := d.decodeFunc(d, rvUnsafePointer(&fv), bs); err != nil {
			f.Reset()
			return false, err
		}
		return true, nil
	}

	// Fallback to reflect-based decoding
	if d.arrayLen < 0 { // slice
		fv.Set(reflect.MakeSlice(fv.Type(), len(bs), len(bs)))
	}
	var s string
	// Determine if we need to copy strings for safety:
	// - If decoding string elements AND data came from non-string sources, use string(v) to create a safe copy
	// - Otherwise, use b2s(v) for zero-copy conversion (safe when data comes from string sources)
	needcopy := d.needcopy && !ctx.IsStr()
	for i, v := range bs {
		rv := fv.Index(i)
		if needcopy {
			s = string(v)
		} else {
			s = b2s(v)
		}
		if err := d.elemDecodeFunc(rv, s); err != nil {
			f.Reset()
			return false, err
		}
	}
	return true, nil
}

var type2DecodeSliceFunc = map[reflect.Type]decodeSliceFunc{
	reflect.TypeOf([]int{}):     decodeIntSlice,
	reflect.TypeOf([]int8{}):    decodeInt8Slice,
	reflect.TypeOf([]int16{}):   decodeInt16Slice,
	reflect.TypeOf([]int32{}):   decodeInt32Slice,
	reflect.TypeOf([]int64{}):   decodeInt64Slice,
	reflect.TypeOf([]uint{}):    decodeUintSlice,
	reflect.TypeOf([]uint8{}):   decodeUint8Slice,
	reflect.TypeOf([]uint16{}):  decodeUint16Slice,
	reflect.TypeOf([]uint32{}):  decodeUint32Slice,
	reflect.TypeOf([]uint64{}):  decodeUint64Slice,
	reflect.TypeOf([]float32{}): decodeFloat32Slice,
	reflect.TypeOf([]float64{}): decodeFloat64Slice,
	reflect.TypeOf([]bool{}):    decodeBoolSlice,
}

// Int slices
func decodeIntSlice(d *sliceDecoder, p unsafe.Pointer, vv [][]byte) error {
	slice := make([]int, len(vv))
	*(*[]int)(p) = slice
	for i, v := range vv {
		val, err := strconv.Atoi(b2s(v))
		if err != nil {
			return err
		}
		slice[i] = val
	}
	return nil
}

func decodeInt8Slice(d *sliceDecoder, p unsafe.Pointer, vv [][]byte) error {
	slice := make([]int8, len(vv))
	*(*[]int8)(p) = slice
	for i, v := range vv {
		val, err := strconv.ParseInt(b2s(v), 10, 8)
		if err != nil {
			return err
		}
		slice[i] = int8(val)
	}
	return nil
}

func decodeInt16Slice(d *sliceDecoder, p unsafe.Pointer, vv [][]byte) error {
	slice := make([]int16, len(vv))
	*(*[]int16)(p) = slice
	for i, v := range vv {
		val, err := strconv.ParseInt(b2s(v), 10, 16)
		if err != nil {
			return err
		}
		slice[i] = int16(val)
	}
	return nil
}

func decodeInt32Slice(d *sliceDecoder, p unsafe.Pointer, vv [][]byte) error {
	slice := make([]int32, len(vv))
	*(*[]int32)(p) = slice
	for i, v := range vv {
		val, err := strconv.ParseInt(b2s(v), 10, 32)
		if err != nil {
			return err
		}
		slice[i] = int32(val)
	}
	return nil
}

func decodeInt64Slice(d *sliceDecoder, p unsafe.Pointer, vv [][]byte) error {
	slice := make([]int64, len(vv))
	*(*[]int64)(p) = slice
	for i, v := range vv {
		val, err := strconv.ParseInt(b2s(v), 10, 64)
		if err != nil {
			return err
		}
		slice[i] = val
	}
	return nil
}

// Unsigned int slices
func decodeUintSlice(d *sliceDecoder, p unsafe.Pointer, vv [][]byte) error {
	slice := make([]uint, len(vv))
	*(*[]uint)(p) = slice
	for i, v := range vv {
		val, err := strconv.ParseUint(b2s(v), 10, 0)
		if err != nil {
			return err
		}
		slice[i] = uint(val)
	}
	return nil
}

func decodeUint8Slice(d *sliceDecoder, p unsafe.Pointer, vv [][]byte) error {
	slice := make([]uint8, len(vv))
	*(*[]uint8)(p) = slice
	for i, v := range vv {
		val, err := strconv.ParseUint(b2s(v), 10, 8)
		if err != nil {
			return err
		}
		slice[i] = uint8(val)
	}
	return nil
}

func decodeUint16Slice(d *sliceDecoder, p unsafe.Pointer, vv [][]byte) error {
	slice := make([]uint16, len(vv))
	*(*[]uint16)(p) = slice
	for i, v := range vv {
		val, err := strconv.ParseUint(b2s(v), 10, 16)
		if err != nil {
			return err
		}
		slice[i] = uint16(val)
	}
	return nil
}

func decodeUint32Slice(d *sliceDecoder, p unsafe.Pointer, vv [][]byte) error {
	slice := make([]uint32, len(vv))
	*(*[]uint32)(p) = slice
	for i, v := range vv {
		val, err := strconv.ParseUint(b2s(v), 10, 32)
		if err != nil {
			return err
		}
		slice[i] = uint32(val)
	}
	return nil
}

func decodeUint64Slice(d *sliceDecoder, p unsafe.Pointer, vv [][]byte) error {
	slice := make([]uint64, len(vv))
	*(*[]uint64)(p) = slice
	for i, v := range vv {
		val, err := strconv.ParseUint(b2s(v), 10, 64)
		if err != nil {
			return err
		}
		slice[i] = val
	}
	return nil
}

// Float slices
func decodeFloat32Slice(d *sliceDecoder, p unsafe.Pointer, vv [][]byte) error {
	slice := make([]float32, len(vv))
	*(*[]float32)(p) = slice
	for i, v := range vv {
		val, err := strconv.ParseFloat(b2s(v), 32)
		if err != nil {
			return err
		}
		slice[i] = float32(val)
	}
	return nil
}

func decodeFloat64Slice(d *sliceDecoder, p unsafe.Pointer, vv [][]byte) error {
	slice := make([]float64, len(vv))
	*(*[]float64)(p) = slice
	for i, v := range vv {
		val, err := strconv.ParseFloat(b2s(v), 64)
		if err != nil {
			return err
		}
		slice[i] = val
	}
	return nil
}

// Bool slice
func decodeBoolSlice(d *sliceDecoder, p unsafe.Pointer, vv [][]byte) error {
	slice := make([]bool, len(vv))
	*(*[]bool)(p) = slice
	for i, v := range vv {
		val, err := strconv.ParseBool(b2s(v))
		if err != nil {
			return err
		}
		slice[i] = val
	}
	return nil
}
