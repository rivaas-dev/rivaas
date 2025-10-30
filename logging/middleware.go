package logging

import (
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// Constants for middleware configuration
const (
	// Pool capacities for performance tuning
	defaultAttrCapacity = 32 // Pre-allocate for typical log entries

	// HTTP status code ranges
	statusOKStart    = 200
	statusWarnStart  = 400
	statusErrorStart = 500
)

// middlewareConfig holds configuration for HTTP logging middleware.
type middlewareConfig struct {
	skipPaths  map[string]bool
	logHeaders bool
}

// Object pools for performance optimization.
// These pools reduce allocations in the hot path by reusing objects.
var (
	responseWriterPool = sync.Pool{
		New: func() any {
			return &responseWriter{}
		},
	}

	contextLoggerPool = sync.Pool{
		New: func() any {
			return &ContextLogger{}
		},
	}

	attrSlicePool = sync.Pool{
		New: func() any {
			// Pre-allocate with reasonable capacity for typical log entries
			s := make([]any, 0, defaultAttrCapacity)
			return &s
		},
	}
)

// MiddlewareOption configures the HTTP logging middleware.
type MiddlewareOption func(*middlewareConfig)

// WithSkipPaths configures paths that should not be logged.
// Useful for health check and metrics endpoints that create log noise.
//
// Example:
//
//	logging.WithSkipPaths("/health", "/metrics", "/readiness")
func WithSkipPaths(paths ...string) MiddlewareOption {
	return func(cfg *middlewareConfig) {
		if cfg.skipPaths == nil {
			cfg.skipPaths = make(map[string]bool)
		}
		for _, p := range paths {
			cfg.skipPaths[p] = true
		}
	}
}

// WithLogHeaders enables logging of HTTP request headers.
// Default: false (headers are not logged for security/privacy).
func WithLogHeaders(enabled bool) MiddlewareOption {
	return func(cfg *middlewareConfig) {
		cfg.logHeaders = enabled
	}
}

// responseWriter wraps http.ResponseWriter to capture status code and response size.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
	written    bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.ResponseWriter.WriteHeader(code)
		rw.written = true
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.written = true
	}
	if rw.statusCode == 0 {
		rw.statusCode = http.StatusOK
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.size += n
	return n, err
}

// StatusCode returns the HTTP status code.
func (rw *responseWriter) StatusCode() int {
	if rw.statusCode == 0 {
		return http.StatusOK
	}
	return rw.statusCode
}

// Size returns the response size in bytes.
func (rw *responseWriter) Size() int {
	return rw.size
}

// reset resets the responseWriter for reuse from the pool.
func (rw *responseWriter) reset(w http.ResponseWriter) {
	rw.ResponseWriter = w
	rw.statusCode = 0
	rw.size = 0
	rw.written = false
}

// Middleware creates HTTP logging middleware that logs request and response details.
// It automatically correlates logs with OpenTelemetry traces if tracing is enabled.
//
// Example:
//
//	logger := logging.MustNew(logging.WithJSONHandler())
//	mw := logging.Middleware(logger,
//	    logging.WithSkipPaths("/health", "/metrics"),
//	    logging.WithLogHeaders(false),
//	)
//	http.Handle("/", mw(myHandler))
func Middleware(cfg *Config, opts ...MiddlewareOption) func(http.Handler) http.Handler {
	mc := &middlewareConfig{
		skipPaths: make(map[string]bool),
	}
	for _, opt := range opts {
		opt(mc)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip logging for configured paths
			if mc.skipPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()

			// Get pooled responseWriter
			rw := responseWriterPool.Get().(*responseWriter)
			rw.reset(w)
			defer responseWriterPool.Put(rw)

			// Get pooled ContextLogger
			cl := contextLoggerPool.Get().(*ContextLogger)
			cl.reset(cfg, r.Context())
			defer contextLoggerPool.Put(cl)

			// Get pooled attribute slice
			attrsPtr := attrSlicePool.Get().(*[]any)
			attrs := (*attrsPtr)[:0]
			defer func() {
				// Reset slice and return pointer to pool
				*attrsPtr = (*attrsPtr)[:0]
				attrSlicePool.Put(attrsPtr)
			}()

			// Build request attributes
			attrs = append(attrs,
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("remote", r.RemoteAddr),
				slog.String("user_agent", r.UserAgent()),
			)

			// Add query parameters if present
			if r.URL.RawQuery != "" {
				attrs = append(attrs, slog.String("query", r.URL.RawQuery))
			}

			// Optionally log headers
			if mc.logHeaders {
				for k, v := range r.Header {
					if len(v) > 0 {
						attrs = append(attrs, slog.String("hdr."+k, v[0]))
					}
				}
			}

			// Log request start
			cl.Info("request started", attrs...)

			// Process request
			next.ServeHTTP(rw, r)

			// Log response (reuse the attrs slice)
			attrs = attrs[:0] // Reset for reuse
			dur := time.Since(start)
			status := rw.StatusCode()

			attrs = append(attrs,
				slog.Int("status", status),
				slog.Int("size", rw.Size()),
				slog.Duration("duration", dur),
			)

			// Choose log level based on status code
			switch {
			case status >= statusErrorStart:
				cl.Error("request completed", attrs...)
			case status >= statusWarnStart:
				cl.Warn("request completed", attrs...)
			default:
				cl.Info("request completed", attrs...)
			}
		})
	}
}
