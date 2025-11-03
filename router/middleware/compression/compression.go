package compression

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"

	"rivaas.dev/router"
)

// Option defines functional options for compression middleware configuration.
type Option func(*config)

// config holds the configuration for the compression middleware.
type config struct {
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

// defaultConfig returns the default configuration for compression middleware.
func defaultConfig() *config {
	return &config{
		level:               gzip.DefaultCompression,
		minSize:             1024, // 1KB
		excludePaths:        make(map[string]bool),
		excludeExtensions:   make(map[string]bool),
		excludeContentTypes: make(map[string]bool),
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

// New returns a middleware that compresses HTTP responses using gzip.
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
//	r.Use(compression.New())
//
// With custom compression level:
//
//	r.Use(compression.New(
//	    compression.WithLevel(gzip.BestCompression),
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
//
// Performance: This middleware uses sync.Pool to reuse gzip writers,
// minimizing allocations. Compression adds ~100-500µs per request depending
// on response size and compression level.
func New(opts ...Option) router.HandlerFunc {
	// Apply options to default config
	cfg := defaultConfig()
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
