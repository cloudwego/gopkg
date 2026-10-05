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
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetter(t *testing.T) {
	result := newGetResult()

	{ // path
		ctx := newTestContextFromRequest(httptest.NewRequest("GET", "http://example.com/", nil))
		path(ctx, "key", result)
		assert.Nil(t, result.Value())

		ctx = newTestContextWithParams("key", "value")
		path(ctx, "key", result)
		assert.NotNil(t, result.Value())
		assert.Equal(t, "value", string(result.Value()))
	}

	{ // form
		ctx := newTestContextFromRequest(httptest.NewRequest("GET", "http://example.com/", nil))
		form(ctx, "key", result)
		assert.Nil(t, result.Value())

		// post
		ctx = newTestContextWithPostForm("key=value")
		form(ctx, "key", result)
		assert.NotNil(t, result.Value())
		assert.Equal(t, "value", string(result.Value()))

		// query
		ctx = newTestContextWithQuery("k=v")
		form(ctx, "k", result)
		assert.NotNil(t, result.Value())
		assert.Equal(t, "v", string(result.Value()))
	}

	{ // query
		ctx := newTestContextFromRequest(httptest.NewRequest("GET", "http://example.com/", nil))
		query(ctx, "key", result)
		assert.Nil(t, result.Value())

		ctx = newTestContextWithQuery("k=v")
		query(ctx, "k", result)
		assert.NotNil(t, result.Value())
		assert.Equal(t, "v", string(result.Value()))
	}

	{ // cookie
		ctx := newTestContextFromRequest(httptest.NewRequest("GET", "http://example.com/", nil))
		cookie(ctx, "key", result)
		assert.Nil(t, result.Value())

		ctx = newTestContextWithCookie("cookiek", "cookiev")
		cookie(ctx, "cookiek", result)
		assert.NotNil(t, result.Value())
		assert.Equal(t, "cookiev", string(result.Value()))
	}

	{ // header
		ctx := newTestContextFromRequest(httptest.NewRequest("GET", "http://example.com/", nil))
		header(ctx, "key", result)
		assert.Nil(t, result.Value())

		ctx = newTestContextWithHeader("hk", "hv")
		header(ctx, "hk", result)
		assert.NotNil(t, result.Value())
		assert.Equal(t, "hv", string(result.Value()))
	}
}

func TestGetterSlice(t *testing.T) {
	result := newGetResult()

	{ // form (now returns all values)
		ctx := newTestContextFromRequest(httptest.NewRequest("GET", "http://example.com/", nil))
		form(ctx, "key", result)
		assert.Equal(t, 0, len(result.Values()))

		// post
		ctx = newTestContextWithPostForm("key=value")
		form(ctx, "key", result)
		assert.NotNil(t, result.Values())
		assert.Equal(t, [][]byte{[]byte("value")}, result.Values())

		// query fallback
		ctx = newTestContextWithQuery("k=v")
		form(ctx, "k", result)
		assert.NotNil(t, result.Values())
		assert.Equal(t, [][]byte{[]byte("v")}, result.Values())
	}

	{ // query (now returns all values)
		ctx := newTestContextFromRequest(httptest.NewRequest("GET", "http://example.com/", nil))
		query(ctx, "key", result)
		assert.Equal(t, 0, len(result.Values()))

		ctx = newTestContextWithQuery("k=v")
		query(ctx, "k", result)
		assert.NotNil(t, result.Values())
		assert.Equal(t, [][]byte{[]byte("v")}, result.Values())
	}

	{ // header (now returns all values)
		ctx := newTestContextFromRequest(httptest.NewRequest("GET", "http://example.com/", nil))
		header(ctx, "key", result)
		assert.Equal(t, 0, len(result.Values()))

		ctx = newTestContextWithHeader("Hk", "hv")
		header(ctx, "Hk", result)
		assert.NotNil(t, result.Values())
		assert.Equal(t, [][]byte{[]byte("hv")}, result.Values())
	}

}
