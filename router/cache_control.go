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

package router

import (
	"fmt"
	"strings"
	"time"
)

// CacheControlOption defines functional options for Cache-Control header configuration.
type CacheControlOption func(*cacheControlConfig)

// cacheControlConfig holds the configuration for Cache-Control directives.
type cacheControlConfig struct {
	public               bool
	private              bool
	noStore              bool
	noCache              bool
	maxAge               time.Duration
	staleWhileRevalidate time.Duration
	staleIfError         time.Duration
}

// WithPublic sets the public directive, allowing shared caches to cache the response.
//
// Example:
//
//	c.CacheControl(WithPublic(), WithMaxAge(time.Minute))
func WithPublic() CacheControlOption {
	return func(cfg *cacheControlConfig) {
		cfg.public = true
	}
}

// WithPrivate sets the private directive, preventing shared caches from caching the response.
//
// Example:
//
//	c.CacheControl(WithPrivate(), WithMaxAge(time.Minute))
func WithPrivate() CacheControlOption {
	return func(cfg *cacheControlConfig) {
		cfg.private = true
	}
}

// WithNoStore sets the no-store directive, preventing any cache from storing the response.
//
// Example:
//
//	c.CacheControl(WithNoStore())
func WithNoStore() CacheControlOption {
	return func(cfg *cacheControlConfig) {
		cfg.noStore = true
	}
}

// WithNoCache sets the no-cache directive, requiring validation before using cached response.
//
// Example:
//
//	c.CacheControl(WithNoCache())
func WithNoCache() CacheControlOption {
	return func(cfg *cacheControlConfig) {
		cfg.noCache = true
	}
}

// WithMaxAge sets the max-age directive in seconds.
//
// Example:
//
//	c.CacheControl(WithMaxAge(5 * time.Minute))
func WithMaxAge(duration time.Duration) CacheControlOption {
	return func(cfg *cacheControlConfig) {
		if duration > 0 {
			cfg.maxAge = duration
		}
	}
}

// WithStaleWhileRevalidate sets the stale-while-revalidate directive in seconds (RFC 5861).
// Allows serving stale content while revalidating in the background.
//
// Example:
//
//	c.CacheControl(WithMaxAge(time.Minute), WithStaleWhileRevalidate(2 * time.Minute))
func WithStaleWhileRevalidate(duration time.Duration) CacheControlOption {
	return func(cfg *cacheControlConfig) {
		if duration > 0 {
			cfg.staleWhileRevalidate = duration
		}
	}
}

// WithStaleIfError sets the stale-if-error directive in seconds.
// Allows serving stale content if the origin server returns an error.
//
// Example:
//
//	c.CacheControl(WithMaxAge(time.Minute), WithStaleIfError(5 * time.Minute))
func WithStaleIfError(duration time.Duration) CacheControlOption {
	return func(cfg *cacheControlConfig) {
		if duration > 0 {
			cfg.staleIfError = duration
		}
	}
}

// CacheControl sets the Cache-Control header based on the provided options.
// Builds a valid Cache-Control header value from the functional options.
//
// Example:
//
//	c.CacheControl(
//	    WithPublic(),
//	    WithMaxAge(time.Minute),
//	    WithStaleWhileRevalidate(2 * time.Minute),
//	)
//	// Sets: Cache-Control: public, max-age=60, stale-while-revalidate=120
func (c *Context) CacheControl(opts ...CacheControlOption) {
	cfg := &cacheControlConfig{}

	for _, opt := range opts {
		opt(cfg)
	}

	parts := make([]string, 0, 7)

	if cfg.public {
		parts = append(parts, "public")
	}
	if cfg.private {
		parts = append(parts, "private")
	}
	if cfg.noStore {
		parts = append(parts, "no-store")
	}
	if cfg.noCache {
		parts = append(parts, "no-cache")
	}
	if cfg.maxAge > 0 {
		parts = append(parts, fmt.Sprintf("max-age=%d", int(cfg.maxAge.Seconds())))
	}
	if cfg.staleWhileRevalidate > 0 {
		parts = append(parts, fmt.Sprintf("stale-while-revalidate=%d", int(cfg.staleWhileRevalidate.Seconds())))
	}
	if cfg.staleIfError > 0 {
		parts = append(parts, fmt.Sprintf("stale-if-error=%d", int(cfg.staleIfError.Seconds())))
	}

	if len(parts) > 0 {
		c.Header("Cache-Control", strings.Join(parts, ", "))
	}
}
