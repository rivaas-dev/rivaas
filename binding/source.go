// Copyright 2025 The Rivaas Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package binding

import (
	"net/http"
	"net/url"
	"strings"
)

// Tag name constants for struct tags used in binding.
const (
	TagJSON    = "json"    // JSON struct tag
	TagQuery   = "query"   // Query parameter struct tag
	TagPath    = "path"    // URL path parameter struct tag
	TagForm    = "form"    // Form data struct tag
	TagHeader  = "header"  // HTTP header struct tag
	TagCookie  = "cookie"  // Cookie struct tag
	TagXML     = "xml"     // XML struct tag
	TagYAML    = "yaml"    // YAML struct tag
	TagTOML    = "toml"    // TOML struct tag
	TagMsgPack = "msgpack" // MessagePack struct tag
	TagProto   = "proto"   // Protocol Buffers struct tag
)

// ValueGetter abstracts different sources of input values for binding.
//
// Implementers must distinguish between "key present with empty value" and
// "key not present". For example:
//   - Query string "?name=" → Has("name") = true, Get("name") = ""
//   - Query string "?foo=bar" → Has("name") = false
//
// This distinction enables proper partial update semantics and default value
// application. The Has method should return true if the key exists in the
// source, even if its value is empty.
//
// ValueGetter is the low-level interface for custom binding sources.
// For built-in sources, use the type-safe functions: [Query], [Path], [Form], etc.
// Use [Raw] or [RawInto] to bind from a custom ValueGetter implementation.
type ValueGetter interface {
	// Get returns the first value for the given key, or an empty string if not present.
	Get(key string) string

	// GetAll returns all values for the given key, or nil if not present.
	GetAll(key string) []string

	// Has returns true if the key is present, even if its value is empty.
	// This distinguishes "key present with empty value" from "key not present".
	Has(key string) bool
}

// approxSizer is an optional interface for [ValueGetter] implementations that
// can estimate the number of keys matching a prefix. This is used for map
// capacity estimation to improve performance when binding map fields.
type approxSizer interface {
	ApproxLen(prefix string) int
}

// GetterFunc is a function adapter that implements [ValueGetter].
// It allows using a function directly as a ValueGetter without creating
// a custom type.
//
// Example:
//
//	getter := binding.GetterFunc(func(key string) ([]string, bool) {
//	    if val, ok := myMap[key]; ok {
//	        return []string{val}, true
//	    }
//	    return nil, false
//	})
//	err := binding.Raw(getter, "custom", &result)
type GetterFunc func(key string) (values []string, has bool)

// Get returns the first value for the key.
func (f GetterFunc) Get(key string) string {
	values, has := f(key)
	if has && len(values) > 0 {
		return values[0]
	}

	return ""
}

// GetAll returns all values for the key.
func (f GetterFunc) GetAll(key string) []string {
	values, _ := f(key)
	return values
}

// Has returns whether the key exists.
func (f GetterFunc) Has(key string) bool {
	_, has := f(key)
	return has
}

// QueryGetter implements [ValueGetter] for URL query parameters.
type QueryGetter struct {
	values url.Values
}

// NewQueryGetter creates a [QueryGetter] from url.Values.
//
// Example:
//
//	getter := binding.NewQueryGetter(r.URL.Query())
//	err := binding.Raw(getter, "query", &result)
func NewQueryGetter(v url.Values) *QueryGetter {
	return &QueryGetter{values: v}
}

// Get returns the first value for the key.
func (q *QueryGetter) Get(key string) string {
	return q.values.Get(key)
}

// GetAll returns all values for the key.
// It supports both repeated key patterns ("ids=1&ids=2") and bracket notation
// ("ids[]=1&ids[]=2").
func (q *QueryGetter) GetAll(key string) []string {
	// Try standard form first
	if vals := q.values[key]; len(vals) > 0 {
		return vals
	}
	// Try bracket notation
	return q.values[key+"[]"]
}

// Has returns whether the key exists.
func (q *QueryGetter) Has(key string) bool {
	return q.values.Has(key) || q.values.Has(key+"[]")
}

// ApproxLen estimates the number of keys starting with the given prefix.
// It checks both dot notation (prefix.) and bracket notation (prefix[).
func (q *QueryGetter) ApproxLen(prefix string) int {
	count := 0
	prefixDot := prefix + "."
	prefixBracket := prefix + "["

	for key := range q.values {
		if strings.HasPrefix(key, prefixDot) || strings.HasPrefix(key, prefixBracket) {
			count++
		}
	}

	return count
}

// PathGetter implements ValueGetter for URL path parameters.
type PathGetter struct {
	params map[string]string
}

// NewPathGetter creates a PathGetter from a map of path parameters.
//
// Example:
//
//	getter := binding.NewPathGetter(map[string]string{"id": "123"})
func NewPathGetter(p map[string]string) *PathGetter {
	return &PathGetter{params: p}
}

// Get returns the value for the key.
func (p *PathGetter) Get(key string) string {
	return p.params[key]
}

// GetAll returns all values for the key as a slice.
// Path parameters are single-valued, so this returns a slice with one element
// if the key exists.
func (p *PathGetter) GetAll(key string) []string {
	if val, ok := p.params[key]; ok {
		return []string{val}
	}

	return nil
}

// Has returns whether the key exists.
func (p *PathGetter) Has(key string) bool {
	_, ok := p.params[key]
	return ok
}

// FormGetter implements [ValueGetter] for form data.
type FormGetter struct {
	values url.Values
}

// NewFormGetter creates a [FormGetter] from url.Values.
//
// Example:
//
//	getter := binding.NewFormGetter(r.PostForm)
//	err := binding.Raw(getter, "form", &result)
func NewFormGetter(v url.Values) *FormGetter {
	return &FormGetter{values: v}
}

// Get returns the first value for the key.
func (f *FormGetter) Get(key string) string {
	return f.values.Get(key)
}

// GetAll returns all values for the key.
// It supports both repeated key patterns ("ids=1&ids=2") and bracket notation
// ("ids[]=1&ids[]=2").
func (f *FormGetter) GetAll(key string) []string {
	// Try standard form first
	if vals := f.values[key]; len(vals) > 0 {
		return vals
	}
	// Try bracket notation
	return f.values[key+"[]"]
}

// Has returns whether the key exists.
func (f *FormGetter) Has(key string) bool {
	return f.values.Has(key) || f.values.Has(key+"[]")
}

// ApproxLen estimates the number of keys starting with the given prefix.
// It checks both dot notation (prefix.) and bracket notation (prefix[).
func (f *FormGetter) ApproxLen(prefix string) int {
	count := 0
	prefixDot := prefix + "."
	prefixBracket := prefix + "["

	for key := range f.values {
		if strings.HasPrefix(key, prefixDot) || strings.HasPrefix(key, prefixBracket) {
			count++
		}
	}

	return count
}

// CookieGetter implements [ValueGetter] for HTTP cookies.
// Cookie names are case-sensitive per HTTP standard.
type CookieGetter struct {
	cookies []*http.Cookie
}

// NewCookieGetter creates a [CookieGetter] from a slice of HTTP cookies.
//
// Example:
//
//	getter := binding.NewCookieGetter(r.Cookies())
//	err := binding.Raw(getter, "cookie", &result)
func NewCookieGetter(c []*http.Cookie) *CookieGetter {
	return &CookieGetter{cookies: c}
}

// Get returns the first cookie value for the key.
// Cookie values are automatically URL-unescaped. If unescaping fails, the
// raw cookie value is returned.
func (cg *CookieGetter) Get(key string) string {
	for _, cookie := range cg.cookies {
		if cookie.Name == key { // Case-sensitive (standard behavior)
			if val, err := url.QueryUnescape(cookie.Value); err == nil {
				return val
			}

			return cookie.Value
		}
	}

	return ""
}

// GetAll returns all cookie values for the key.
func (cg *CookieGetter) GetAll(key string) []string {
	var values []string
	for _, cookie := range cg.cookies {
		if cookie.Name == key {
			if val, err := url.QueryUnescape(cookie.Value); err == nil {
				values = append(values, val)
			} else {
				values = append(values, cookie.Value)
			}
		}
	}

	return values
}

// Has returns whether the key exists.
func (cg *CookieGetter) Has(key string) bool {
	for _, cookie := range cg.cookies {
		if cookie.Name == key {
			return true
		}
	}

	return false
}

// HeaderGetter implements [ValueGetter] for HTTP headers.
// Headers are case-insensitive per HTTP standard, and keys are canonicalized
// using http.CanonicalHeaderKey.
type HeaderGetter struct {
	headers    http.Header
	normalized map[string]string // Canonical key -> first value
}

// NewHeaderGetter creates a [HeaderGetter] from http.Header.
// Header keys are normalized to canonical MIME header format for consistent lookups.
//
// Example:
//
//	getter := binding.NewHeaderGetter(r.Header)
//	err := binding.Raw(getter, "header", &result)
func NewHeaderGetter(h http.Header) *HeaderGetter {
	// Headers are already canonicalized by http.Header, but we store
	// a normalized map for consistent lookups
	normalized := make(map[string]string, len(h))
	for key, values := range h {
		if len(values) > 0 {
			normalized[http.CanonicalHeaderKey(key)] = values[0]
		}
	}

	return &HeaderGetter{headers: h, normalized: normalized}
}

// Get returns the first header value for the key.
// Lookups are case-insensitive and use canonical header key format.
func (h *HeaderGetter) Get(key string) string {
	return h.normalized[http.CanonicalHeaderKey(key)]
}

// GetAll returns all header values for the key.
func (h *HeaderGetter) GetAll(key string) []string {
	return h.headers.Values(key)
}

// Has returns whether the key exists.
func (h *HeaderGetter) Has(key string) bool {
	_, ok := h.normalized[http.CanonicalHeaderKey(key)]
	return ok
}
