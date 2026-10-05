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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// testContextWrapper wraps RequestContext to add path parameter support for testing.
// It embeds RequestContext to forward all methods except GetPathValue.
type testContextWrapper struct {
	RequestContext
	pathParams map[string]string
}

// newTestContextWithParams creates a wrapper with path parameters for testing.
func newTestContextWithParams(kvs ...string) *testContextWrapper {
	req := httptest.NewRequest("GET", "http://example.com/", nil)
	ctx := NewHTTPRequestContext(req)
	wrapper := &testContextWrapper{
		RequestContext: ctx,
		pathParams:     make(map[string]string),
	}
	for i := 0; i < len(kvs); i += 2 {
		wrapper.pathParams[kvs[i]] = kvs[i+1]
	}
	return wrapper
}

// GetPathValue returns path parameters set for testing.
func (t *testContextWrapper) GetPathValue(key string, result *GetResult) {
	result.Reset()
	if v, ok := t.pathParams[key]; ok {
		result.SetStr(v)
	}
}

// Helper functions for creating test contexts without path parameters

// newTestContextFromRequest creates a RequestContext from an http.Request for testing.
func newTestContextFromRequest(req *http.Request) RequestContext {
	return NewHTTPRequestContext(req)
}

// newTestContextWithQuery creates a RequestContext with query parameters for testing.
func newTestContextWithQuery(query string) RequestContext {
	req := httptest.NewRequest("GET", "http://example.com/?"+query, nil)
	return newTestContextFromRequest(req)
}

// newTestContextWithPostForm creates a RequestContext with POST form data for testing.
func newTestContextWithPostForm(formData string) RequestContext {
	req := httptest.NewRequest("POST", "http://example.com/", bytes.NewBufferString(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return newTestContextFromRequest(req)
}

// newTestContextWithHeader creates a RequestContext with headers for testing.
func newTestContextWithHeader(key, value string) RequestContext {
	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.Header.Set(key, value)
	return newTestContextFromRequest(req)
}

// newTestContextWithCookie creates a RequestContext with a cookie for testing.
func newTestContextWithCookie(name, value string) RequestContext {
	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.AddCookie(&http.Cookie{Name: name, Value: value})
	return newTestContextFromRequest(req)
}

// newTestContextWithBody creates a RequestContext with a request body for testing.
func newTestContextWithBody(body string) RequestContext {
	req := httptest.NewRequest("POST", "http://example.com/", bytes.NewBufferString(body))
	return newTestContextFromRequest(req)
}

func TestHTTPRequestContext(t *testing.T) {
	t.Run("Path", func(t *testing.T) {
		req := httptest.NewRequest("GET", "http://example.com/users/123", nil)
		ctx := NewHTTPRequestContext(req)

		result := newGetResult()
		ctx.GetPathValue("id", result)
		assert.Nil(t, result.Value())
	})

	t.Run("Query", func(t *testing.T) {
		ctx := newTestContextWithQuery("name=alice&tag=a&tag=b")
		result := newGetResult()

		// Test GetQuery (single value)
		ctx.GetQuery("name", result)
		assert.NotNil(t, result.Value())
		assert.Equal(t, []byte("alice"), result.Value())

		// Test GetQuery with missing key
		ctx.GetQuery("missing", result)
		assert.Nil(t, result.Value())

		// Test GetQuery with multiple values (now returns all)
		ctx.GetQuery("tag", result)
		assert.NotNil(t, result.Values())
		assert.Equal(t, [][]byte{[]byte("a"), []byte("b")}, result.Values())

		// Test GetQuery with missing key
		ctx.GetQuery("missing", result)
		assert.Equal(t, 0, len(result.Values()))
	})

	t.Run("Header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "http://example.com/", nil)
		req.Header.Set("User-Agent", "test-agent")
		req.Header.Add("Accept", "application/json")
		req.Header.Add("Accept", "text/plain")
		ctx := NewHTTPRequestContext(req)
		result := newGetResult()

		// Test GetHeader (single value)
		ctx.GetHeader("User-Agent", result)
		assert.NotNil(t, result.Value())
		assert.Equal(t, []byte("test-agent"), result.Value())

		// Test GetHeader with missing key
		ctx.GetHeader("Missing", result)
		assert.Nil(t, result.Value())

		// Test GetHeader with multiple values (now returns all)
		ctx.GetHeader("Accept", result)
		assert.NotNil(t, result.Values())
		assert.Equal(t, [][]byte{[]byte("application/json"), []byte("text/plain")}, result.Values())

		// Test GetHeader with missing key
		ctx.GetHeader("Missing", result)
		assert.Equal(t, 0, len(result.Values()))
	})

	t.Run("PostForm", func(t *testing.T) {
		ctx := newTestContextWithPostForm("username=alice&role=admin&role=user")
		result := newGetResult()

		// Test GetPostForm (single value)
		ctx.GetPostForm("username", result)
		assert.NotNil(t, result.Value())
		assert.Equal(t, []byte("alice"), result.Value())

		// Test GetPostForm with missing key
		ctx.GetPostForm("missing", result)
		assert.Nil(t, result.Value())

		// Test GetPostForm with multiple values (now returns all)
		ctx.GetPostForm("role", result)
		assert.NotNil(t, result.Values())
		assert.Equal(t, [][]byte{[]byte("admin"), []byte("user")}, result.Values())

		// Test GetPostForm with missing key
		ctx.GetPostForm("missing", result)
		assert.Equal(t, 0, len(result.Values()))
	})

	t.Run("Cookie", func(t *testing.T) {
		ctx := newTestContextWithCookie("session", "abc123")
		result := newGetResult()

		// Test GetCookie
		ctx.GetCookie("session", result)
		assert.NotNil(t, result.Value())
		assert.Equal(t, []byte("abc123"), result.Value())

		// Test GetCookie with missing key
		ctx.GetCookie("missing", result)
		assert.Nil(t, result.Value())
	})

	t.Run("Body", func(t *testing.T) {
		body := "test body content"
		ctx := newTestContextWithBody(body)

		// Test GetBody
		b := ctx.GetBody()
		assert.Equal(t, []byte(body), b)

		// Body can be read multiple times
		b = ctx.GetBody()
		assert.Equal(t, []byte(body), b)
	})

	t.Run("Context", func(t *testing.T) {
		req := httptest.NewRequest("GET", "http://example.com/", nil)
		ctx := NewHTTPRequestContext(req)

		// Test GetContext
		requestCtx := ctx.GetContext()
		assert.NotNil(t, requestCtx)
		assert.Equal(t, req.Context(), requestCtx)
	})
}
