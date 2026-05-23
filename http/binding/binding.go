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
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
)

// Decoder decodes request data into a struct.
// The Decode method binds values from RequestContext into the provided value v,
// which must be a pointer to a struct.
type Decoder interface {
	Decode(req RequestContext, v any) (bool, error)
}

type decodeCtx struct {
	RequestContext
	GetResult
}

// decodeCtxPool is a memory pool for decodeCtx instances to reduce allocations
var decodeCtxPool = sync.Pool{
	New: func() interface{} {
		return &decodeCtx{
			GetResult: GetResult{
				vv: make([][]byte, 0, 4),
			},
		}
	},
}

// getDecodeCtx retrieves a decodeCtx from the pool (internal use only).
func getDecodeCtx(req RequestContext) *decodeCtx {
	ctx := decodeCtxPool.Get().(*decodeCtx)
	ctx.RequestContext = req
	return ctx
}

// releaseDecodeCtx returns a decodeCtx to the pool after resetting it (internal use only).
func releaseDecodeCtx(ctx *decodeCtx) {
	if ctx == nil {
		return
	}
	ctx.GetResult.Reset()
	ctx.RequestContext = nil
	decodeCtxPool.Put(ctx)
}

type fieldDecoder interface {
	Decode(ctx *decodeCtx, rv reflect.Value) (bool, error)
	GetFieldName() string
}

type DecodeConfig struct {
	// JSONUnmarshalFunc is the function used for JSON unmarshaling
	// If nil, will use encoding/json.Unmarshal as default
	JSONUnmarshalFunc func(data []byte, v interface{}) error

	// Tags specifies the tags to use for decoding in order of preference.
	// If not set (nil or empty), the default tags are used: path, form, query, cookie, header
	// If set (e.g., []string{"form", "query"}), only the specified tags are used in the given order.
	Tags []string
}

func (c *DecodeConfig) getJSONUnmarshal() func(data []byte, v interface{}) error {
	if c.JSONUnmarshalFunc != nil {
		return c.JSONUnmarshalFunc
	}
	// Default to encoding/json
	return json.Unmarshal
}

// NewDecoder creates a new Decoder for the given struct type.
// The rt parameter must be a pointer to struct type (e.g., reflect.TypeOf((*MyStruct)(nil))).
// The config parameter specifies decoding behavior (tags, JSON unmarshaler, etc.).
// If config is nil, default configuration is used.
//
// Supported struct tags (in default priority order):
//   - path: binds from path parameters
//   - form: binds from POST form data, falls back to query parameters
//   - query: binds from URL query parameters
//   - cookie: binds from HTTP cookies
//   - header: binds from HTTP headers
//
// Returns an error if rt is not a pointer to struct type.
func NewDecoder(rt reflect.Type, config *DecodeConfig) (Decoder, error) {
	if rt.Kind() != reflect.Pointer {
		return nil, errors.New("not pointer type")
	}
	rt = rt.Elem()
	if rt.Kind() != reflect.Struct {
		return nil, fmt.Errorf("unsupported %s type binding", rt)
	}
	if config == nil {
		config = &DecodeConfig{}
	}
	return newStructDecoder(rt, config)
}

func getFieldDecoder(fi *fieldInfo) (fieldDecoder, error) {
	ft := fi.fieldType

	fp := reflect.PointerTo(ft)
	// Priority: UnmarshalParam (custom) > TextUnmarshaler (standard) > base types
	if fp.Implements(paramUnmarshalerType) {
		return newUnmarshalParamDecoder(fi), nil
	}
	if fp.Implements(textUnmarshalerType) {
		return newTextUnmarshalerDecoder(fi), nil
	}

	switch ft.Kind() {
	case reflect.Slice, reflect.Array:
		elemType := dereferenceType(ft.Elem())
		// Check if it's a file slice
		if elemType == fileBindingType {
			return newFileTypeSliceDecoder(fi), nil
		}

		ep := reflect.PointerTo(elemType)
		// Check if element type implements UnmarshalParam
		if ep.Implements(paramUnmarshalerType) {
			return newUnmarshalParamSliceDecoder(fi), nil
		}
		// Check if element type implements TextUnmarshaler
		if ep.Implements(textUnmarshalerType) {
			return newTextUnmarshalerSliceDecoder(fi), nil
		}
		return newSliceDecoder(fi), nil

	case reflect.Struct:
		if ft == fileBindingType {
			return newFileTypeDecoder(fi), nil
		}
	}
	return newBaseDecoder(fi), nil
}

type textUnmarshaler interface {
	UnmarshalText(text []byte) error
}

var textUnmarshalerType = reflect.TypeOf((*textUnmarshaler)(nil)).Elem()

type paramUnmarshaler interface {
	UnmarshalParam(param string) error
}

var paramUnmarshalerType = reflect.TypeOf((*paramUnmarshaler)(nil)).Elem()
