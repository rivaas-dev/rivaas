package binding

import (
	"net/http"
	"net/url"
	"strings"
)

// Tag name constants (avoids stringly-typed mistakes).
const (
	TagJSON   = "json"
	TagQuery  = "query"
	TagParams = "params"
	TagForm   = "form"
	TagHeader = "header"
	TagCookie = "cookie"
)

// ValueGetter abstracts different sources of input values.
//
// Implementers MUST distinguish between "key present with empty value" and
// "key not present". For example:
//   - Query string "?name=" → Has("name") = true, Get("name") = ""
//   - Query string "?foo=bar" → Has("name") = false
//
// This distinction enables proper partial update semantics and default value
// application. The Has() method should return true if the key exists in the
// source, even if its value is empty.
type ValueGetter interface {
	// Get returns the first value for the given key, or "" if not present.
	Get(key string) string

	// GetAll returns all values for the given key, or nil if not present.
	GetAll(key string) []string

	// Has returns true if the key is present (even if value is empty).
	// This is used to distinguish "key present with empty value" from "key not present".
	Has(key string) bool
}

// approxSizer is an optional capability for getters that can estimate
// the number of keys matching a prefix (for map pre-allocation).
type approxSizer interface {
	ApproxLen(prefix string) int
}

// GetterFunc is a function adapter for ValueGetter (inlining-friendly).
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

// QueryGetter implements ValueGetter for URL query parameters.
type QueryGetter struct {
	values url.Values
}

// NewQueryGetter creates a new QueryGetter.
func NewQueryGetter(v url.Values) ValueGetter {
	return &QueryGetter{values: v}
}

// Get returns the first value for the key.
func (q *QueryGetter) Get(key string) string {
	return q.values.Get(key)
}

// GetAll returns all values for the key.
// Supports both "ids=1&ids=2" and "ids[]=1&ids[]=2" patterns.
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

// ApproxLen estimates the number of keys starting with prefix.
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

// ParamsGetter implements ValueGetter for URL path parameters.
type ParamsGetter struct {
	params map[string]string
}

// NewParamsGetter creates a new ParamsGetter.
func NewParamsGetter(p map[string]string) ValueGetter {
	return &ParamsGetter{params: p}
}

// Get returns the value for the key.
func (p *ParamsGetter) Get(key string) string {
	return p.params[key]
}

// GetAll returns all values for the key (single value as slice).
func (p *ParamsGetter) GetAll(key string) []string {
	if val, ok := p.params[key]; ok {
		return []string{val}
	}
	return nil
}

// Has returns whether the key exists.
func (p *ParamsGetter) Has(key string) bool {
	_, ok := p.params[key]
	return ok
}

// FormGetter implements ValueGetter for form data.
type FormGetter struct {
	values url.Values
}

// NewFormGetter creates a new FormGetter.
func NewFormGetter(v url.Values) ValueGetter {
	return &FormGetter{values: v}
}

// Get returns the first value for the key.
func (f *FormGetter) Get(key string) string {
	return f.values.Get(key)
}

// GetAll returns all values for the key.
// Supports both "ids=1&ids=2" and "ids[]=1&ids[]=2" patterns.
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

// ApproxLen estimates the number of keys starting with prefix.
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

// CookieGetter implements ValueGetter for cookies.
// Cookie names are case-sensitive (standard HTTP behavior).
type CookieGetter struct {
	cookies []*http.Cookie
}

// NewCookieGetter creates a new CookieGetter.
func NewCookieGetter(c []*http.Cookie) ValueGetter {
	return &CookieGetter{cookies: c}
}

// Get returns the first cookie value for the key.
// Cookie values are automatically URL-unescaped.
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

// HeaderGetter implements ValueGetter for HTTP headers.
// Headers are case-insensitive (http.Header.Get canonicalizes).
type HeaderGetter struct {
	headers    http.Header
	normalized map[string]string // Canonical key -> first value
}

// NewHeaderGetter creates a new HeaderGetter with canonical MIME header key normalization.
func NewHeaderGetter(h http.Header) ValueGetter {
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

// Get returns the first header value for the key (case-insensitive).
// Always uses canonical form for lookups.
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
