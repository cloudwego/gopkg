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
	"context"
	"io"
	"mime/multipart"
	"net/http"
)

// RequestContext abstracts access to request data for binding/decoding.
// It provides a clean interface to retrieve values from different sources
// (path params, query params, headers, form data, body, etc.)
type RequestContext interface {
	// GetContext retrieves the request context
	GetContext() context.Context

	// GetPathValue retrieves a path parameter by key, populating result with the value
	GetPathValue(key string, result *GetResult)

	// GetQuery retrieves all query parameter values by key, populating result with the values
	GetQuery(key string, result *GetResult)

	// GetHeader retrieves all header values by key, populating result with the values
	GetHeader(key string, result *GetResult)

	// GetPostForm retrieves all form parameter values by key, populating result with the values
	GetPostForm(key string, result *GetResult)

	// GetCookie retrieves a cookie value by key, populating result with the value
	GetCookie(key string, result *GetResult)

	// GetBody retrieves the raw request body
	GetBody() []byte

	// GetFormFiles retrieves file(s) from multipart form by key
	GetFormFiles(key string) ([]*multipart.FileHeader, error)
}

// GetResult holds values fetched from request and tracks their data source.
// It stores byte slices and marks whether they came from string sources (headers, query params)
// or binary sources. This helps optimize string conversions in decoders.
type GetResult struct {
	vv  [][]byte
	// str is true when values came from string sources (headers, query params, cookies, etc.)
	// This allows decoders to use b2s() for zero-copy string conversion when safe
	str bool
}

func (p *GetResult) Value() []byte {
	if len(p.vv) > 0 {
		return p.vv[0]
	}
	return nil
}

func (p *GetResult) Values() [][]byte {
	return p.vv
}

// IsStr returns true if the values came from string sources.
// When true, decoders can safely use b2s() for zero-copy conversion.
// When false (data from binary sources), a proper copy with string(v) is needed for string fields.
func (p *GetResult) IsStr() bool {
	return p.str
}

// SetStr sets string values and marks them as coming from string sources.
// Uses s2b() for zero-copy conversion, so str flag is set to true.
func (p *GetResult) SetStr(ss ...string) {
	p.vv = p.vv[:0]
	for _, s := range ss {
		p.vv = append(p.vv, s2b(s))
	}
	p.str = true
}

// Set stores byte values and marks them as coming from non-string (binary) sources.
// str flag is set to false, indicating string(v) copy is needed for string fields.
func (p *GetResult) Set(vv ...[]byte) {
	p.vv = append(p.vv[:0], vv...)
	p.str = false
}

func (p *GetResult) Reset() {
	p.vv = p.vv[:0]
}

func newGetResult() *GetResult {
	return &GetResult{}
}

// httpRequestContext wraps http.Request to implement RequestContext interface.
// It provides a clean interface to retrieve values from different sources:
// - path parameters (from path params via http.Request.PathValue, Go 1.22+)
// - query parameters (from URL query string, lazily cached)
// - headers (from HTTP headers)
// - form data (from POST form data)
// - cookies (from HTTP cookies)
// - body (raw request body, cached for multiple reads)
// - multipart files (from multipart form uploads)
type httpRequestContext struct {
	req        *http.Request
	body       []byte // cached body for multiple reads
	cachedURLv map[string][]string
}

// NewHTTPRequestContext creates a new RequestContext from an http.Request.
// The request body is not read until GetBody() is called, allowing lazy loading.
// Note: The returned context uses s2b (unsafe string-to-bytes conversion) for performance.
// Returned byte slices share memory with request strings and are valid only for the request lifetime.
func NewHTTPRequestContext(req *http.Request) RequestContext {
	return &httpRequestContext{req: req}
}

// GetContext retrieves the request context
func (h *httpRequestContext) GetContext() context.Context {
	return h.req.Context()
}

// GetPathValue retrieves a path parameter by key using http.Request.PathValue (Go 1.22+).
func (h *httpRequestContext) GetPathValue(key string, result *GetResult) {
	// PathValue is available in Go 1.22+
	type pathValueProvider interface {
		PathValue(key string) string
	}
	if pvp, ok := interface{}(h.req).(pathValueProvider); ok {
		if v := pvp.PathValue(key); v != "" {
			result.SetStr(v)
		}
	}
}

// GetQuery retrieves all query parameter values by key.
// Query string is lazily parsed and cached on first call.
func (h *httpRequestContext) GetQuery(key string, result *GetResult) {
	result.Reset()
	if h.cachedURLv == nil {
		h.cachedURLv = h.req.URL.Query()
	}
	values := h.cachedURLv[key]
	if len(values) > 0 {
		result.SetStr(values...)
	}
}

// GetHeader retrieves all header values by key.
func (h *httpRequestContext) GetHeader(key string, result *GetResult) {
	result.Reset()
	values := h.req.Header.Values(key)
	if len(values) > 0 {
		result.SetStr(values...)
	}
}

// GetPostForm retrieves all form parameter values by key.
func (h *httpRequestContext) GetPostForm(key string, result *GetResult) {
	result.Reset()
	if err := h.req.ParseForm(); err != nil {
		return
	}
	values := h.req.PostForm[key]
	if len(values) > 0 {
		result.SetStr(values...)
	}
}

// GetCookie retrieves a cookie value by key.
func (h *httpRequestContext) GetCookie(key string, result *GetResult) {
	result.Reset()
	cookie, err := h.req.Cookie(key)
	if err == nil && cookie.Value != "" {
		result.SetStr(cookie.Value)
	}
}

// GetBody retrieves the cached request body (lazily loaded on first call).
func (h *httpRequestContext) GetBody() []byte {
	if h.body != nil || h.req.Body == nil {
		return h.body
	}
	r := h.req
	if r.Body == nil {
		return nil
	}
	body, _ := io.ReadAll(r.Body)
	h.body = body

	// Reset Body for potential re-reads
	if len(body) > 0 {
		r.Body = io.NopCloser(bytes.NewReader(body))
	}
	return body
}

// GetFormFiles retrieves multipart form files by key.
func (h *httpRequestContext) GetFormFiles(key string) ([]*multipart.FileHeader, error) {
	if err := h.req.ParseMultipartForm(32 << 20); err != nil {
		return nil, err
	}
	if h.req.MultipartForm == nil {
		return nil, nil
	}
	if files, ok := h.req.MultipartForm.File[key]; ok {
		return files, nil
	}
	return nil, nil
}
