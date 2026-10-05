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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewTagInfo(t *testing.T) {
	ti := newTagInfo("query", "id")
	assert.Equal(t, "query", ti.Tag)
	assert.Equal(t, "id", ti.Name)
	assert.NotNil(t, ti.Getter)
}

func TestTagInfoParse(t *testing.T) {
	ti := newTagInfo("form", "")

	{ // Basic name
		ti.Parse("username")
		assert.Equal(t, "username", ti.Name)
		assert.Empty(t, ti.Options)
	}

	{ // Name with options
		ti.Parse("email,required,max=100")
		assert.Equal(t, "email", ti.Name)
		assert.Len(t, ti.Options, 2)
		assert.Equal(t, "required", ti.Options[0])
		assert.Equal(t, "max=100", ti.Options[1])
	}

	{ // Empty name (preserves existing)
		ti.Parse(",opt1,opt2")
		assert.Equal(t, "email", ti.Name) // unchanged
		assert.Len(t, ti.Options, 2)
		assert.Equal(t, "opt1", ti.Options[0])
		assert.Equal(t, "opt2", ti.Options[1])
	}

	{ // Single option
		ti.Parse("field,single")
		assert.Equal(t, "field", ti.Name)
		assert.Len(t, ti.Options, 1)
		assert.Equal(t, "single", ti.Options[0])
	}
}

func TestTagInfoSkip(t *testing.T) {
	{ // Skip field
		ti := newTagInfo("path", "-")
		assert.True(t, ti.Skip())
	}

	{ // Don't skip
		ti := newTagInfo("path", "id")
		assert.False(t, ti.Skip())
	}
}

func TestLookupFieldTags(t *testing.T) {
	type TestStruct struct {
		ID    string `path:"id"`
		Name  string `form:"name" query:"name"`
		Email string `header:"x-email"`
		Skip  string `path:"-"`
		Other string // no tags
	}

	field, _ := reflect.TypeOf((*TestStruct)(nil)).Elem().FieldByName("ID")
	tagInfos := lookupFieldTags(field, &DecodeConfig{})
	assert.Len(t, tagInfos, 1)
	assert.Equal(t, "path", tagInfos[0].Tag)
	assert.Equal(t, "id", tagInfos[0].Name)

	field, _ = reflect.TypeOf((*TestStruct)(nil)).Elem().FieldByName("Name")
	tagInfos = lookupFieldTags(field, &DecodeConfig{})
	assert.Len(t, tagInfos, 2)
	assert.Equal(t, "form", tagInfos[0].Tag)
	assert.Equal(t, "name", tagInfos[0].Name)
	assert.Equal(t, "query", tagInfos[1].Tag)
	assert.Equal(t, "name", tagInfos[1].Name)

	field, _ = reflect.TypeOf((*TestStruct)(nil)).Elem().FieldByName("Other")
	tagInfos = lookupFieldTags(field, &DecodeConfig{})
	assert.Empty(t, tagInfos)
}
