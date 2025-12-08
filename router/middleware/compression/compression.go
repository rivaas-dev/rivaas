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

import (
	"compress/gzip"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/andybalholm/brotli"

	"rivaas.dev/router"
)

// Option defines functional options for compression middleware configuration.
type Option func(*config)

// config holds the configuration for the compression middleware.
type config struct {
	// logger is the structured logger for error logging (slog from standard library)
	logger *slog.Logger

	// gzipLevel is the gzip compression level (0-9, where 0=no compression, 9=best compression)
	gzipLevel int

	// brotliLevel is the Brotli compression level (0-11)
	// For dynamic content (JSON/text), use 4-5. Higher levels are CPU-expensive.
	brotliLevel int

	// minSize is the minimum response size to compress (in bytes)
	minSize int

	// enableGzip enables gzip compression
	enableGzip bool

	// enableBrotli enables Brotli compression
	enableBrotli bool

	// excludePaths are paths that should not be compressed
	excludePaths map[string]bool

	// excludeExtensions are file extensions that should not be compressed
	excludeExtensions map[string]bool

	// excludeContentTypes are content types that should not be compressed
	excludeContentTypes map[string]bool
}

// defaultConfig returns the default configuration for compression middleware.
func defaultConfig() *config {
	return &config{
		gzipLevel:           gzip.DefaultCompression,
		brotliLevel:         4, // Conservative for dynamic content
		minSize:             0, // 0 = no threshold, compress all supported responses
		enableGzip:          true,
		enableBrotli:        true,
		excludePaths:        make(map[string]bool),
		excludeExtensions:   make(map[string]bool),
		excludeContentTypes: make(map[string]bool),
	}
}

// compressWriter wraps the response writer to compress the response body.
// It buffers data up to the threshold before deciding whether to compress.
type compressWriter struct {
	http.ResponseWriter
	writer              io.WriteCloser
	pool                *sync.Pool
	encoding            string
	excludeContentTypes map[string]bool
	threshold           int

	buffer      []byte // Buffer for threshold check
	bufferUsed  int
	headersSent bool
	statusCode  int
	decided     bool
	compress    bool
}

// Write buffers data and decides on compression based on threshold.
func (cw *compressWriter) Write(data []byte) (int, error) {
	// If already decided, write directly
	if cw.decided {
		if cw.compress {
			return cw.writer.Write(data)
		}

		return cw.ResponseWriter.Write(data)
	}

	// If threshold is 0, compress immediately without buffering
	if cw.threshold == 0 {
		cw.decided = true
		cw.compress = true
		cw.initCompression()

		return cw.writer.Write(data)
	}

	// Buffer until we hit threshold
	originalLen := len(data)
	space := cap(cw.buffer) - len(cw.buffer)
	if space > 0 {
		toCopy := min(space, len(data))
		cw.buffer = append(cw.buffer, data[:toCopy]...)
		cw.bufferUsed += toCopy
		data = data[toCopy:]
	}

	// Decision time: compress if threshold reached, otherwise write uncompressed
	if cw.bufferUsed >= cw.threshold || len(data) > 0 {
		cw.decided = true

		if cw.bufferUsed >= cw.threshold {
			return cw.writeCompressed(data)
		}

		return cw.writeUncompressed(data)
	}

	return originalLen, nil
}

// writeCompressed initializes compression and writes all data through the compressor.
func (cw *compressWriter) writeCompressed(data []byte) (int, error) {
	cw.compress = true
	cw.initCompression()

	return cw.flushBufferAndWrite(cw.writer, data)
}

// writeUncompressed writes all data directly without compression.
func (cw *compressWriter) writeUncompressed(data []byte) (int, error) {
	cw.compress = false
	if !cw.headersSent {
		cw.ResponseWriter.WriteHeader(cw.statusCode)
	}

	return cw.flushBufferAndWrite(cw.ResponseWriter, data)
}

// flushBufferAndWrite writes buffered data then remaining data to the given writer.
func (cw *compressWriter) flushBufferAndWrite(w io.Writer, data []byte) (int, error) {
	written := 0

	if cw.bufferUsed > 0 {
		n, err := w.Write(cw.buffer)
		written += n
		if err != nil {
			return written, err
		}
	}

	if len(data) > 0 {
		n, err := w.Write(data)
		written += n

		return written, err
	}

	return written, nil
}

// WriteHeader captures the status code and checks if compression should be skipped.
func (cw *compressWriter) WriteHeader(code int) {
	if cw.headersSent {
		return
	}

	cw.statusCode = code

	// Don't compress these status codes
	if shouldSkipStatus(code) {
		cw.compress = false
		cw.decided = true
		cw.ResponseWriter.WriteHeader(code)
		cw.headersSent = true

		return
	}

	// Check content type
	contentType := cw.ResponseWriter.Header().Get("Content-Type")
	if shouldSkipContentType(contentType, cw.excludeContentTypes) {
		cw.compress = false
		cw.decided = true
		cw.ResponseWriter.WriteHeader(code)
		cw.headersSent = true

		return
	}

	// Don't call underlying WriteHeader yet - wait for decision
	// We'll call it when we decide or when first write happens
	// Note: headersSent remains false until we actually send headers
}

// initCompression initializes the compression writer and sets headers.
func (cw *compressWriter) initCompression() {
	// Set headers
	cw.ResponseWriter.Header().Del("Content-Length")
	cw.ResponseWriter.Header().Set("Content-Encoding", cw.encoding)
	cw.ResponseWriter.Header().Set("Vary", "Accept-Encoding")

	if !cw.headersSent {
		cw.ResponseWriter.WriteHeader(cw.statusCode)
		cw.headersSent = true
	}

	// Get writer from pool
	switch cw.encoding {
	case "br":
		w := cw.pool.Get().(*brotli.Writer)
		w.Reset(cw.ResponseWriter)
		cw.writer = w
	case "gzip":
		w := cw.pool.Get().(*gzip.Writer)
		w.Reset(cw.ResponseWriter)
		cw.writer = w
	}
}

// Close finalizes compression and returns writers to pools.
func (cw *compressWriter) Close() error {
	if !cw.decided {
		// Small response that never exceeded threshold
		cw.decided = true
		cw.compress = false
		if cw.bufferUsed > 0 {
			if !cw.headersSent {
				cw.ResponseWriter.WriteHeader(cw.statusCode)
			}
			cw.ResponseWriter.Write(cw.buffer)
		}

		return nil
	}

	if cw.compress && cw.writer != nil {
		err := cw.writer.Close()
		// Reset before returning to pool to reduce holding references
		switch w := cw.writer.(type) {
		case *brotli.Writer:
			w.Reset(nil)
		case *gzip.Writer:
			w.Reset(nil)
		}
		cw.pool.Put(cw.writer)

		return err
	}

	return nil
}

// shouldSkipStatus returns true if the status code should not be compressed.
func shouldSkipStatus(code int) bool {
	return code == http.StatusNoContent ||
		code == http.StatusNotModified ||
		code == http.StatusPartialContent
}

// shouldSkipContentType returns true if the content type should not be compressed.
func shouldSkipContentType(ct string, excludes map[string]bool) bool {
	if ct == "" {
		return false
	}

	// Always skip these
	ctLower := strings.ToLower(ct)
	if strings.Contains(ctLower, "text/event-stream") ||
		strings.Contains(ctLower, "application/grpc") ||
		strings.Contains(ctLower, "application/octet-stream") {

		return true
	}

	// Check user exclusions
	for excluded := range excludes {
		if strings.Contains(ctLower, strings.ToLower(excluded)) {
			return true
		}
	}

	return false
}

// gzipWriterPool is a sync.Pool for reusing gzip writers.
var (
	gzipWriterPools   = make(map[int]*sync.Pool)
	brotliWriterPools = make(map[int]*sync.Pool)
	poolsMutex        sync.RWMutex
)

// getGzipWriterPool returns a pool for the specified compression level.
func getGzipWriterPool(level int) *sync.Pool {
	poolsMutex.RLock()
	pool, exists := gzipWriterPools[level]
	poolsMutex.RUnlock()

	if exists {
		return pool
	}

	poolsMutex.Lock()
	defer poolsMutex.Unlock()

	// Double-check after acquiring write lock
	if pool, exists := gzipWriterPools[level]; exists {
		return pool
	}

	pool = &sync.Pool{
		New: func() any {
			w, _ := gzip.NewWriterLevel(io.Discard, level)
			return w
		},
	}
	gzipWriterPools[level] = pool

	return pool
}

// getBrotliWriterPool returns a pool for the specified Brotli compression level.
func getBrotliWriterPool(level int) *sync.Pool {
	poolsMutex.RLock()
	pool, exists := brotliWriterPools[level]
	poolsMutex.RUnlock()

	if exists {
		return pool
	}

	poolsMutex.Lock()
	defer poolsMutex.Unlock()

	// Double-check after acquiring write lock
	if pool, exists := brotliWriterPools[level]; exists {
		return pool
	}

	pool = &sync.Pool{
		New: func() any {
			return brotli.NewWriterLevel(io.Discard, level)
		},
	}
	brotliWriterPools[level] = pool

	return pool
}

// chooseEncoding selects the best encoding based on Accept-Encoding header.
// Respects q-values and prefers Brotli over gzip if both are available.
func chooseEncoding(acceptEncoding string, cfg *config) string {
	if acceptEncoding == "" {
		return ""
	}

	ae := strings.ToLower(acceptEncoding)

	brQ := parseQValue(ae, "br")
	gzipQ := parseQValue(ae, "gzip")

	// Explicit q=0 means not acceptable
	if brQ == 0 && gzipQ == 0 {
		return ""
	}

	// Prefer Brotli if enabled and quality is equal or better
	if cfg.enableBrotli && brQ > 0 && brQ >= gzipQ {
		return "br"
	}

	if cfg.enableGzip && gzipQ > 0 {
		return "gzip"
	}

	return ""
}

// parseQValue returns -1 if not present, 0 if q=0, or the parsed quality value.
func parseQValue(accept, encoding string) float64 {
	idx := strings.Index(accept, encoding)
	if idx < 0 {
		return -1
	}

	qIdx := strings.Index(accept[idx:], "q=")
	if qIdx < 0 {
		return 1.0
	}

	qStart := idx + qIdx + 2
	end := strings.IndexAny(accept[qStart:], ",;")
	if end < 0 {
		end = len(accept) - qStart
	}

	qStr := strings.TrimSpace(accept[qStart : qStart+end])
	q, err := strconv.ParseFloat(qStr, 64)
	if err != nil {
		return 1.0
	}

	return q
}

// New returns a middleware that compresses HTTP responses using gzip and/or Brotli.
// It automatically detects client support and selects the best encoding based on
// Accept-Encoding header with q-value negotiation.
//
// Features:
//   - Automatic gzip and Brotli compression with quality-value negotiation
//   - Configurable compression levels for both algorithms
//   - Minimum size threshold with buffering to avoid compressing small responses
//   - Path and content-type exclusions
//   - Writer pooling for reduced allocations
//   - Skips compression for 204, 304, 206, SSE, and gRPC
//   - Sets Vary: Accept-Encoding header
//   - Respects existing Content-Encoding headers (proxying)
//
// Basic usage:
//
//	r := router.MustNew()
//	r.Use(compression.New())
//
// With custom compression levels:
//
//	r.Use(compression.New(
//	    compression.WithGzipLevel(gzip.BestCompression),
//	    compression.WithBrotliLevel(5),
//	))
//
// Disable Brotli (gzip only):
//
//	r.Use(compression.New(
//	    compression.WithBrotliDisabled(),
//	))
//
// Exclude certain paths:
//
//	r.Use(compression.New(
//	    compression.WithExcludePaths("/metrics", "/stream"),
//	))
//
// Exclude already compressed formats:
//
//	r.Use(compression.New(
//	    compression.WithExcludeExtensions(".jpg", ".png", ".gif", ".zip"),
//	    compression.WithExcludeContentTypes("image/jpeg", "image/png"),
//	))
func New(opts ...Option) router.HandlerFunc {
	// Apply options to default config
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	return func(c *router.Context) {
		// Early exit: path excluded
		if cfg.excludePaths[c.Request.URL.Path] {
			c.Next()
			return
		}

		// Early exit: extension excluded
		path := c.Request.URL.Path
		for ext := range cfg.excludeExtensions {
			if strings.HasSuffix(path, ext) {
				c.Next()
				return
			}
		}

		// Early exit: already compressed
		if c.Response.Header().Get("Content-Encoding") != "" {
			c.Next()
			return
		}

		// Early exit: no client support
		encoding := chooseEncoding(c.Request.Header.Get("Accept-Encoding"), cfg)
		if encoding == "" {
			c.Next()
			return
		}

		// Get appropriate pool
		var pool *sync.Pool
		switch encoding {
		case "br":
			pool = getBrotliWriterPool(cfg.brotliLevel)
		case "gzip":
			pool = getGzipWriterPool(cfg.gzipLevel)
		default:
			c.Next()
			return
		}

		// Wrap response writer
		// Only allocate buffer if threshold is set (> 0)
		var buf []byte
		if cfg.minSize > 0 {
			buf = make([]byte, 0, cfg.minSize)
		}
		cw := &compressWriter{
			ResponseWriter:      c.Response,
			encoding:            encoding,
			excludeContentTypes: cfg.excludeContentTypes,
			threshold:           cfg.minSize,
			buffer:              buf,
			pool:                pool,
		}

		originalWriter := c.Response
		c.Response = cw

		c.Next()

		// Finalize
		if err := cw.Close(); err != nil {
			if cfg.logger != nil {
				cfg.logger.Error("compression finalization failed", "error", err)
			}
		}

		c.Response = originalWriter
	}
}
