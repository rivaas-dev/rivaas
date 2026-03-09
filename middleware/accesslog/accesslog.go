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
	"crypto/sha256"
	"encoding/binary"
	"math/rand/v2"
	"strings"
	"time"

	"rivaas.dev/router"
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
			wrapped := router.NewResponseWriterWrapper(c.Response)
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
				if cfg.requestIDFunc != nil {
					// Deterministic: hash-based sampling by request ID
					shouldLog = sampleByHash(cfg.requestIDFunc(c), cfg.sampleRate)
				} else {
					// Random: probabilistic sampling (not security-sensitive)
					//nolint:gosec // G404: Using math/rand/v2 for sampling is appropriate here
					shouldLog = rand.Float64() < cfg.sampleRate
				}
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
			logger.Error("http request", fields...)
		case status >= 400:
			logger.Warn("http request", fields...)
		case isSlow:
			logger.Warn("http request", fields...)
		default:
			logger.Info("http request", fields...)
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
