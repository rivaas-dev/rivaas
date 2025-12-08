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

package accesslog

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"rivaas.dev/router"
	"rivaas.dev/router/middleware"
)

// statusSizer is a capability interface for response writers that track status/size.
// This avoids coupling to internal router types.
type statusSizer interface {
	StatusCode() int
	Size() int64
}

// New creates an access log middleware with structured logging.
//
// The logger must be provided via WithLogger option. If no logger is configured,
// the middleware will skip logging.
//
// Example:
//
//	import "log/slog"
//
//	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
//	r := router.MustNew()
//	r.Use(accesslog.New(
//		accesslog.WithLogger(logger),
//		accesslog.WithExcludePaths("/health", "/metrics"),
//		accesslog.WithSlowThreshold(500 * time.Millisecond),
//	))
func New(opts ...Option) router.HandlerFunc {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	return func(c *router.Context) {
		path := c.Request.URL.Path

		// Check exact exclusions
		if cfg.excludePaths[path] {
			c.Next()
			return
		}

		// Check prefix exclusions
		for _, prefix := range cfg.excludePrefixes {
			if strings.HasPrefix(path, prefix) {
				c.Next()
				return
			}
		}

		// CRITICAL FIX: Record start time BEFORE handler
		start := time.Now()

		// Wrap response writer to capture status/size (if not already wrapped)
		var ss statusSizer
		if existing, ok := c.Response.(statusSizer); ok {
			// Already has capability, use it
			ss = existing
		} else {
			// Wrap it
			wrapped := &responseWriter{ResponseWriter: c.Response}
			c.Response = wrapped
			ss = wrapped
		}

		// CRITICAL FIX: Execute handler FIRST
		c.Next()

		// CRITICAL FIX: Decide whether to log AFTER handler (with outcome known)
		duration := time.Since(start)
		status := ss.StatusCode()

		// Sampling decision based on outcome
		shouldLog := true

		// Errors/slow requests bypass sampling (forced logging)
		isError := status >= 400
		isSlow := cfg.slowThreshold > 0 && duration >= cfg.slowThreshold

		if !isError && !isSlow {
			// Normal request - apply filters
			if cfg.logErrorsOnly {
				shouldLog = false
			} else if cfg.sampleRate < 1.0 {
				// Deterministic sampling by request ID hash
				var requestID string
				if v := c.Request.Context().Value(middleware.RequestIDKey); v != nil {
					if rid, ok := v.(string); ok {
						requestID = rid
					}
				}
				shouldLog = sampleByHash(requestID, cfg.sampleRate)
			}
		}

		if !shouldLog {
			return
		}

		// Get logger from config (returns nil if not configured)
		logger := cfg.logger
		if logger == nil {
			// No logger configured, skip logging
			return
		}

		// Build log fields
		fields := []any{
			"method", c.Request.Method,
			"path", path,
			"status", status,
			"duration_ms", duration.Milliseconds(),
			"bytes_sent", ss.Size(),
			"user_agent", c.Request.UserAgent(),
			"client_ip", c.ClientIP(),
			"host", c.Request.Host,
			"proto", c.Request.Proto,
		}

		// Add route pattern (including sentinels)
		if routePattern := c.RoutePattern(); routePattern != "" {
			fields = append(fields, "route", routePattern)
		}

		if isSlow {
			fields = append(fields, "slow", true)
		}

		// Log at appropriate level
		switch {
		case status >= 500:
			logger.Error("access", fields...)
		case status >= 400:
			logger.Warn("access", fields...)
		case isSlow:
			logger.Warn("access", fields...)
		default:
			logger.Info("access", fields...)
		}
	}
}

// sampleByHash provides deterministic sampling based on a hash of the ID.
// Same request ID always makes the same sampling decision across all replicas.
func sampleByHash(id string, rate float64) bool {
	if id == "" {
		return true // No ID, log it
	}

	// Hash the ID to a uint64
	h := sha256.Sum256([]byte(id))
	hashValue := binary.BigEndian.Uint64(h[:8])

	// Deterministic threshold check
	threshold := uint64(rate * float64(^uint64(0)))

	return hashValue <= threshold
}

// responseWriter wraps http.ResponseWriter and preserves optional interfaces.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int64
	written    bool
}

// Compile-time interface checks
var (
	_ http.ResponseWriter = (*responseWriter)(nil)
	_ http.Flusher        = (*responseWriter)(nil)
	_ http.Hijacker       = (*responseWriter)(nil)
	_ http.Pusher         = (*responseWriter)(nil)
	_ io.ReaderFrom       = (*responseWriter)(nil)
)

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.size += int64(n)

	return n, err
}

func (rw *responseWriter) StatusCode() int {
	if rw.statusCode == 0 {
		return http.StatusOK
	}

	return rw.statusCode
}

func (rw *responseWriter) Size() int64 {
	return rw.size
}

// Flush implements http.Flusher
func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack implements http.Hijacker (for WebSocket, etc.)
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}

	return nil, nil, errors.New("hijacker not supported")
}

// Push implements http.Pusher (HTTP/2 server push)
func (rw *responseWriter) Push(target string, opts *http.PushOptions) error {
	if p, ok := rw.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}

	return http.ErrNotSupported
}

// ReadFrom implements io.ReaderFrom using zero-copy when available.
func (rw *responseWriter) ReadFrom(r io.Reader) (int64, error) {
	if rf, ok := rw.ResponseWriter.(io.ReaderFrom); ok {
		n, err := rf.ReadFrom(r)
		rw.size += n

		return n, err
	}
	// Fallback to io.Copy
	n, err := io.Copy(rw.ResponseWriter, r)
	rw.size += n

	return n, err
}
