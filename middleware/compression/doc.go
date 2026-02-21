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

// Package compression provides middleware for HTTP response compression.
//
// This middleware automatically compresses HTTP responses using gzip, deflate,
// or brotli compression algorithms based on client Accept-Encoding headers.
// It reduces bandwidth usage and improves response times for text-based content.
//
// # Basic Usage
//
//	import "rivaas.dev/middleware/compression"
//
//	r := router.MustNew()
//	r.Use(compression.New())
//
// # Supported Algorithms
//
// The middleware supports multiple compression algorithms:
//
//   - gzip: Standard gzip compression (widely supported)
//   - deflate: Deflate compression (legacy support)
//   - brotli: Brotli compression (better compression ratio, modern browsers)
//
// The middleware automatically selects the best algorithm based on client
// Accept-Encoding headers and configured preferences.
//
// # Configuration Options
//
//   - Level: Compression level (1-9, higher = better compression but slower)
//   - MinSize: Minimum response size to compress (default: 1KB)
//   - ContentTypes: Content types to compress (default: text/*, application/json, etc.)
//   - ExcludePaths: Paths to exclude from compression (e.g., /metrics)
//   - Logger: Optional logger for compression events
//
// # Content Type Filtering
//
// By default, the middleware compresses text-based content types:
//
//   - text/*
//   - application/json
//   - application/javascript
//   - application/xml
//   - application/xhtml+xml
//
// Binary content types (images, videos, etc.) are excluded by default.
package compression
