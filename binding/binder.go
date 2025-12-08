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
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Binder provides request data binding with configurable options.
//
// Use [New] or [MustNew] to create a configured Binder, or use package-level
// functions ([Query], [JSON], etc.) for zero-configuration binding.
//
// Binder is safe for concurrent use by multiple goroutines.
//
// Note: Due to Go language limitations, generic methods are not supported.
// Use the generic helper functions [QueryWith], [JSONWith], etc. for generic binding
// with a Binder, or use the non-generic methods directly.
//
// Example:
//
//	binder := binding.MustNew(
//	    binding.WithConverter[uuid.UUID](uuid.Parse),
//	    binding.WithTimeLayouts("2006-01-02"),
//	    binding.WithRequired(),
//	)
//
//	// Generic usage with helper function
//	user, err := binding.JSONWith[CreateUserRequest](binder, body)
//
//	// Non-generic usage
//	var user CreateUserRequest
//	err := binder.JSONTo(body, &user)
type Binder struct {
	cfg *config
}

// New creates a [Binder] with the given options.
// Returns an error if configuration is invalid.
//
// Example:
//
//	binder, err := binding.New(
//	    binding.WithMaxDepth(16),
//	    binding.WithRequired(),
//	)
//	if err != nil {
//	    return fmt.Errorf("failed to create binder: %w", err)
//	}
func New(opts ...Option) (*Binder, error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &Binder{cfg: cfg}, nil
}

// MustNew creates a [Binder] with the given options.
// Panics if configuration is invalid.
//
// Use in main() or init() where panic on startup is acceptable.
//
// Example:
//
//	binder := binding.MustNew(
//	    binding.WithConverter[uuid.UUID](uuid.Parse),
//	    binding.WithTimeLayouts("2006-01-02"),
//	)
func MustNew(opts ...Option) *Binder {
	b, err := New(opts...)
	if err != nil {
		panic(fmt.Sprintf("binding.MustNew: %v", err))
	}

	return b
}

// QueryWith binds URL query parameters to type T using the [Binder]'s config.
//
// Example:
//
//	params, err := binding.QueryWith[ListParams](binder, r.URL.Query())
func QueryWith[T any](b *Binder, values url.Values) (T, error) {
	var result T
	if err := bindFromSource(&result, NewQueryGetter(values), TagQuery, b.cfg); err != nil {
		return result, err
	}

	return result, nil
}

// PathWith binds URL path parameters to type T using the Binder's config.
//
// Example:
//
//	params, err := binding.PathWith[GetUserParams](binder, pathParams)
func PathWith[T any](b *Binder, params map[string]string) (T, error) {
	var result T
	if err := bindFromSource(&result, NewPathGetter(params), TagPath, b.cfg); err != nil {
		return result, err
	}

	return result, nil
}

// FormWith binds form data to type T using the Binder's config.
//
// Example:
//
//	data, err := binding.FormWith[FormData](binder, r.PostForm)
func FormWith[T any](b *Binder, values url.Values) (T, error) {
	var result T
	if err := bindFromSource(&result, NewFormGetter(values), TagForm, b.cfg); err != nil {
		return result, err
	}

	return result, nil
}

// HeaderWith binds HTTP headers to type T using the Binder's config.
//
// Example:
//
//	headers, err := binding.HeaderWith[RequestHeaders](binder, r.Header)
func HeaderWith[T any](b *Binder, h http.Header) (T, error) {
	var result T
	if err := bindFromSource(&result, NewHeaderGetter(h), TagHeader, b.cfg); err != nil {
		return result, err
	}

	return result, nil
}

// CookieWith binds cookies to type T using the Binder's config.
//
// Example:
//
//	session, err := binding.CookieWith[SessionData](binder, r.Cookies())
func CookieWith[T any](b *Binder, cookies []*http.Cookie) (T, error) {
	var result T
	if err := bindFromSource(&result, NewCookieGetter(cookies), TagCookie, b.cfg); err != nil {
		return result, err
	}

	return result, nil
}

// JSONWith binds JSON bytes to type T using the Binder's config.
//
// Example:
//
//	user, err := binding.JSONWith[CreateUserRequest](binder, body)
func JSONWith[T any](b *Binder, body []byte) (T, error) {
	var result T
	if err := bindJSONBytesInternal(&result, body, b.cfg); err != nil {
		return result, err
	}

	return result, nil
}

// JSONReaderWith binds JSON from an io.Reader to type T using the Binder's config.
//
// Example:
//
//	user, err := binding.JSONReaderWith[CreateUserRequest](binder, r.Body)
func JSONReaderWith[T any](b *Binder, r io.Reader) (T, error) {
	var result T
	if err := bindJSONReaderInternal(&result, r, b.cfg); err != nil {
		return result, err
	}

	return result, nil
}

// XMLWith binds XML bytes to type T using the Binder's config.
//
// Example:
//
//	user, err := binding.XMLWith[CreateUserRequest](binder, body)
func XMLWith[T any](b *Binder, body []byte) (T, error) {
	var result T
	if err := bindXMLBytesInternal(&result, body, b.cfg); err != nil {
		return result, err
	}

	return result, nil
}

// XMLReaderWith binds XML from an io.Reader to type T using the Binder's config.
//
// Example:
//
//	user, err := binding.XMLReaderWith[CreateUserRequest](binder, r.Body)
func XMLReaderWith[T any](b *Binder, r io.Reader) (T, error) {
	var result T
	if err := bindXMLReaderInternal(&result, r, b.cfg); err != nil {
		return result, err
	}

	return result, nil
}

// BindWith binds from one or more sources to type T using the Binder's config.
//
// Example:
//
//	req, err := binding.BindWith[CreateOrderRequest](binder,
//	    binding.FromPath(pathParams),
//	    binding.FromQuery(r.URL.Query()),
//	    binding.FromJSON(body),
//	)
func BindWith[T any](b *Binder, opts ...Option) (T, error) {
	var result T
	// Merge binder config with per-call options
	cfg := b.cfg.clone()
	for _, opt := range opts {
		opt(cfg)
	}
	if err := bindMultiSource(&result, cfg); err != nil {
		return result, err
	}

	return result, nil
}

// QueryTo binds URL query parameters to out.
//
// Example:
//
//	var params ListParams
//	err := binder.QueryTo(r.URL.Query(), &params)
func (b *Binder) QueryTo(values url.Values, out any) error {
	return bindFromSource(out, NewQueryGetter(values), TagQuery, b.cfg)
}

// PathTo binds URL path parameters to out.
//
// Example:
//
//	var params GetUserParams
//	err := binder.PathTo(pathParams, &params)
func (b *Binder) PathTo(params map[string]string, out any) error {
	return bindFromSource(out, NewPathGetter(params), TagPath, b.cfg)
}

// FormTo binds form data to out.
//
// Example:
//
//	var data FormData
//	err := binder.FormTo(r.PostForm, &data)
func (b *Binder) FormTo(values url.Values, out any) error {
	return bindFromSource(out, NewFormGetter(values), TagForm, b.cfg)
}

// HeaderTo binds HTTP headers to out.
//
// Example:
//
//	var headers RequestHeaders
//	err := binder.HeaderTo(r.Header, &headers)
func (b *Binder) HeaderTo(h http.Header, out any) error {
	return bindFromSource(out, NewHeaderGetter(h), TagHeader, b.cfg)
}

// CookieTo binds cookies to out.
//
// Example:
//
//	var session SessionData
//	err := binder.CookieTo(r.Cookies(), &session)
func (b *Binder) CookieTo(cookies []*http.Cookie, out any) error {
	return bindFromSource(out, NewCookieGetter(cookies), TagCookie, b.cfg)
}

// JSONTo binds JSON bytes to out.
//
// Example:
//
//	var user CreateUserRequest
//	err := binder.JSONTo(body, &user)
func (b *Binder) JSONTo(body []byte, out any) error {
	return bindJSONBytesInternal(out, body, b.cfg)
}

// JSONReaderTo binds JSON from an io.Reader to out.
//
// Example:
//
//	var user CreateUserRequest
//	err := binder.JSONReaderTo(r.Body, &user)
func (b *Binder) JSONReaderTo(r io.Reader, out any) error {
	return bindJSONReaderInternal(out, r, b.cfg)
}

// XMLTo binds XML bytes to out.
//
// Example:
//
//	var user CreateUserRequest
//	err := binder.XMLTo(body, &user)
func (b *Binder) XMLTo(body []byte, out any) error {
	return bindXMLBytesInternal(out, body, b.cfg)
}

// XMLReaderTo binds XML from an io.Reader to out.
//
// Example:
//
//	var user CreateUserRequest
//	err := binder.XMLReaderTo(r.Body, &user)
func (b *Binder) XMLReaderTo(r io.Reader, out any) error {
	return bindXMLReaderInternal(out, r, b.cfg)
}

// BindTo binds from one or more sources specified via From* options.
//
// Example:
//
//	var req CreateOrderRequest
//	err := binder.BindTo(&req,
//	    binding.FromPath(pathParams),
//	    binding.FromQuery(r.URL.Query()),
//	    binding.FromJSON(body),
//	)
func (b *Binder) BindTo(out any, opts ...Option) error {
	// Merge binder config with per-call options
	cfg := b.cfg.clone()
	for _, opt := range opts {
		opt(cfg)
	}

	return bindMultiSource(out, cfg)
}
