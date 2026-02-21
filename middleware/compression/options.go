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

package compression

import "log/slog"

// WithGzipLevel sets the gzip compression level.
// Valid values: 0 (no compression) to 9 (best compression).
// Default: gzip.DefaultCompression (-1, which is typically level 6)
//
// Example:
//
//	compression.New(compression.WithGzipLevel(gzip.BestCompression))
func WithGzipLevel(level int) Option {
	return func(cfg *config) {
		cfg.gzipLevel = level
	}
}

// WithBrotliLevel sets the Brotli compression level.
// Valid values: 0 (no compression) to 11 (best compression).
// For dynamic content (JSON/text), use 4-5. Higher levels are CPU-expensive.
// Default: 4 (conservative for dynamic content)
//
// Example:
//
//	compression.New(compression.WithBrotliLevel(5))
func WithBrotliLevel(level int) Option {
	return func(cfg *config) {
		// Clamp to valid Brotli level range [0, 11]
		cfg.brotliLevel = max(0, min(level, 11))
	}
}

// WithBrotliDisabled disables Brotli compression (gzip only).
//
// Example:
//
//	compression.New(compression.WithBrotliDisabled())
func WithBrotliDisabled() Option {
	return func(cfg *config) {
		cfg.enableBrotli = false
	}
}

// WithGzipDisabled disables gzip compression (Brotli only).
//
// Example:
//
//	compression.New(compression.WithGzipDisabled())
func WithGzipDisabled() Option {
	return func(cfg *config) {
		cfg.enableGzip = false
	}
}

// WithMinSize sets the minimum response size to compress (in bytes).
//
// DESIGN DECISION: This feature is intentionally NOT implemented.
//
// Why minSize is not implemented:
//   - Requires buffering entire response before compression decision
//   - Adds memory overhead (buffer per request)
//   - Adds latency (wait for full response before sending)
//   - Breaks streaming responses
//   - Modern networks handle small compressed payloads efficiently
//
// Alternative approaches (recommended):
//   - Use WithExcludePaths for small endpoints
//   - Use WithExcludeContentTypes for small data types
//   - Let CDN/reverse proxy handle compression (nginx, CloudFlare)
//
// If you truly need minSize, implement at the reverse proxy level where
// buffering is already happening (nginx, CloudFlare, etc.)
//
// This option exists for API compatibility but is a no-op.
// Default: 1024 (1KB, not enforced)
//
// Example:
//
//	compression.New(compression.WithMinSize(2048)) // No effect (by design)
func WithMinSize(size int) Option {
	return func(cfg *config) {
		cfg.minSize = size
		// NOTE: Intentionally not implemented - see function documentation
	}
}

// WithExcludePaths sets paths that should not be compressed.
// Useful for endpoints that already serve compressed content or streaming responses.
//
// Example:
//
//	compression.New(compression.WithExcludePaths("/metrics", "/stream"))
func WithExcludePaths(paths ...string) Option {
	return func(cfg *config) {
		for _, path := range paths {
			cfg.excludePaths[path] = true
		}
	}
}

// WithExcludeExtensions sets file extensions that should not be compressed.
// Already compressed formats don't benefit from compression.
// Default: none (but should typically exclude .jpg, .png, .gif, .zip, etc.)
//
// Example:
//
//	compression.New(compression.WithExcludeExtensions(".jpg", ".png", ".gif", ".zip", ".gz"))
func WithExcludeExtensions(extensions ...string) Option {
	return func(cfg *config) {
		for _, ext := range extensions {
			cfg.excludeExtensions[ext] = true
		}
	}
}

// WithExcludeContentTypes sets content types that should not be compressed.
// Already compressed content types don't benefit from compression.
//
// Example:
//
//	compression.New(compression.WithExcludeContentTypes("image/jpeg", "image/png", "application/zip"))
func WithExcludeContentTypes(contentTypes ...string) Option {
	return func(cfg *config) {
		for _, ct := range contentTypes {
			cfg.excludeContentTypes[ct] = true
		}
	}
}

// WithLogger sets the slog.Logger for error logging.
// If not provided, errors will be silently ignored.
//
// Uses the standard library's log/slog package for structured logging:
//
//	import "log/slog"
//
//	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
//	compression.New(compression.WithLogger(logger))
//
// Example:
//
//	import "log/slog"
//
//	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
//	r.Use(compression.New(compression.WithLogger(logger)))
func WithLogger(logger *slog.Logger) Option {
	return func(cfg *config) {
		cfg.logger = logger
	}
}
