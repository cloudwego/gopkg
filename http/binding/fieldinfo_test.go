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
	"bytes"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewFieldInfo(t *testing.T) {
	type TestStruct struct {
		IntField int
	}

	structType := reflect.TypeOf((*TestStruct)(nil)).Elem()
	field, _ := structType.FieldByName("IntField")
	config := &DecodeConfig{}

	fi := newFieldInfo(field, config)

	assert.NotNil(t, fi)
	assert.Equal(t, "IntField", fi.fieldName)
	assert.Equal(t, reflect.Int, fi.fieldType.Kind())
	assert.NotNil(t, fi.jsonUnmarshal)
}

func TestFieldInfoFieldValue(t *testing.T) {
	type TestStruct struct {
		ID   int
		Name string
	}

	structType := reflect.TypeOf((*TestStruct)(nil)).Elem()
	config := &DecodeConfig{}

	{ // Int field
		field, _ := structType.FieldByName("ID")
		fi := newFieldInfo(field, config)

		ts := TestStruct{ID: 42, Name: "test"}
		rv := reflect.ValueOf(&ts)
		fieldValue := fi.FieldValue(rv)

		assert.Equal(t, int64(42), fieldValue.Int())
	}

	{ // String field
		field, _ := structType.FieldByName("Name")
		fi := newFieldInfo(field, config)

		ts := TestStruct{ID: 42, Name: "hello"}
		rv := reflect.ValueOf(&ts)
		fieldValue := fi.FieldValue(rv)

		assert.Equal(t, "hello", fieldValue.String())
	}
}

func TestFieldInfoGetFieldName(t *testing.T) {
	type TestStruct struct {
		MyField string
	}

	structType := reflect.TypeOf((*TestStruct)(nil)).Elem()
	field, _ := structType.FieldByName("MyField")
	config := &DecodeConfig{}

	fi := newFieldInfo(field, config)
	assert.Equal(t, "MyField", fi.GetFieldName())
}

func TestFieldInfoFieldSetter(t *testing.T) {
	type TestStruct struct {
		Value int
	}

	structType := reflect.TypeOf((*TestStruct)(nil)).Elem()
	field, _ := structType.FieldByName("Value")
	config := &DecodeConfig{}

	fi := newFieldInfo(field, config)

	ts := TestStruct{Value: 42}
	rv := reflect.ValueOf(&ts)

	// FieldSetter returns fieldSetter which can be used to get/set field values
	setter := fi.FieldSetter(rv)
	fieldValue := setter.Value()
	assert.Equal(t, int64(42), fieldValue.Int())
}

func TestFieldInfoFetchBindValue(t *testing.T) {
	type TestStruct struct {
		ID string `path:"id"`
	}

	structType := reflect.TypeOf((*TestStruct)(nil)).Elem()
	field, _ := structType.FieldByName("ID")
	config := &DecodeConfig{}

	fi := newFieldInfo(field, config)

	// Add tag info
	ti := newTagInfo("path", "id")
	fi.tagInfos = []*tagInfo{ti}

	reqCtx := newTestContextWithParams("id", "123")
	ctx := getDecodeCtx(reqCtx)
	defer releaseDecodeCtx(ctx)

	tag, res := fi.FetchBindValue(ctx)
	assert.Equal(t, "path", tag)
	assert.NotNil(t, res)
	assert.Equal(t, []byte("123"), res)
}

func TestFieldInfoFetchBindValueNotFound(t *testing.T) {
	type TestStruct struct {
		ID string `path:"id"`
	}

	structType := reflect.TypeOf((*TestStruct)(nil)).Elem()
	field, _ := structType.FieldByName("ID")
	config := &DecodeConfig{}

	fi := newFieldInfo(field, config)

	// Add tag info
	ti := newTagInfo("path", "id")
	fi.tagInfos = []*tagInfo{ti}

	// Empty context - no data
	reqCtx := newTestContextWithParams()
	ctx := getDecodeCtx(reqCtx)
	defer releaseDecodeCtx(ctx)

	tag, v := fi.FetchBindValue(ctx)
	assert.Empty(t, tag)
	assert.Empty(t, v)
}

func TestFieldInfoFetchBindValueMultipleTags(t *testing.T) {
	type TestStruct struct {
		Value string `query:"value" form:"value"`
	}

	structType := reflect.TypeOf((*TestStruct)(nil)).Elem()
	field, _ := structType.FieldByName("Value")
	config := &DecodeConfig{}

	fi := newFieldInfo(field, config)

	// Add multiple tag infos
	ti1 := newTagInfo("query", "value")
	ti2 := newTagInfo("form", "value")
	fi.tagInfos = []*tagInfo{ti1, ti2}

	{ // Query parameter takes precedence
		req := httptest.NewRequest("POST", "http://example.com/?value=query_val", bytes.NewBufferString("value=form_val"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		reqCtx := newTestContextFromRequest(req)
		ctx := getDecodeCtx(reqCtx)
		defer releaseDecodeCtx(ctx)

		tag, res := fi.FetchBindValue(ctx)
		assert.Equal(t, "query", tag)
		assert.NotNil(t, res)
		assert.Equal(t, []byte("query_val"), res)
	}

	{ // Falls back to form if query not found
		reqCtx := newTestContextWithPostForm("value=form_val")
		ctx := getDecodeCtx(reqCtx)
		defer releaseDecodeCtx(ctx)

		tag, res := fi.FetchBindValue(ctx)
		assert.Equal(t, "form", tag)
		assert.NotNil(t, res)
		assert.Equal(t, []byte("form_val"), res)
	}
}
