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
	"log/slog"
	"time"
)

// Option defines functional options for access log middleware.
type Option func(*config)

// config holds access log configuration.
type config struct {
	// logger is the structured logger for access logs (slog from standard library)
	logger *slog.Logger

	// excludePaths are exact paths to skip
	excludePaths map[string]bool

	// excludePrefixes are path prefixes to skip (e.g., "/metrics")
	excludePrefixes []string

	// sampleRate samples access logs (1.0 = all, 0.1 = 10%)
	sampleRate float64

	// logErrorsOnly only logs requests with status >= 400
	logErrorsOnly bool

	// slowThreshold logs slow requests separately (forced logging)
	slowThreshold time.Duration
}

func defaultConfig() *config {
	return &config{
		excludePaths:  make(map[string]bool),
		sampleRate:    1.0, // Log everything by default
		logErrorsOnly: false,
	}
}

// WithExcludePaths skips logging for exact path matches.
//
// Example:
//
//	accesslog.New(
//		accesslog.WithExcludePaths("/health", "/metrics"),
//	)
func WithExcludePaths(paths ...string) Option {
	return func(c *config) {
		for _, path := range paths {
			c.excludePaths[path] = true
		}
	}
}

// WithExcludePrefixes skips logging for paths with given prefixes.
//
// Example:
//
//	accesslog.New(
//		accesslog.WithExcludePrefixes("/metrics", "/debug"),
//	)
func WithExcludePrefixes(prefixes ...string) Option {
	return func(c *config) {
		c.excludePrefixes = append(c.excludePrefixes, prefixes...)
	}
}

// WithSampleRate sets the sampling rate (0.0 to 1.0).
// A rate of 1.0 logs all requests, 0.1 logs 10% of requests.
// Sampling is deterministic based on request ID hash.
//
// Example:
//
//	accesslog.New(
//		accesslog.WithSampleRate(0.1), // Log 10% of requests
//	)
func WithSampleRate(rate float64) Option {
	return func(c *config) {
		// Clamp to valid sample rate range [0.0, 1.0]
		c.sampleRate = max(0.0, min(rate, 1.0))
	}
}

// WithErrorsOnly only logs requests with errors (status >= 400).
// This is useful for reducing log volume in production while still
// capturing all errors for debugging.
//
// Example:
//
//	accesslog.New(
//		accesslog.WithErrorsOnly(),
//	)
func WithErrorsOnly() Option {
	return func(c *config) {
		c.logErrorsOnly = true
	}
}

// WithSlowThreshold logs slow requests separately (forced, ignores sampling).
// Requests that exceed the threshold will always be logged, even if
// sampling would normally skip them.
//
// Example:
//
//	accesslog.New(
//		accesslog.WithSlowThreshold(500 * time.Millisecond),
//	)
func WithSlowThreshold(threshold time.Duration) Option {
	return func(c *config) {
		c.slowThreshold = threshold
	}
}

// WithLogger sets the slog.Logger for access logs.
// If not provided, the middleware will skip logging.
//
// Uses the standard library's log/slog package for structured logging:
//
//	import "log/slog"
//
//	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
//	accesslog.New(accesslog.WithLogger(logger))
//
// For production JSON logging:
//
//	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
//		Level: slog.LevelInfo,
//	}))
//	r.Use(accesslog.New(accesslog.WithLogger(logger)))
//
// For development text logging:
//
//	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
//		Level: slog.LevelDebug,
//	}))
//	r.Use(accesslog.New(accesslog.WithLogger(logger)))
func WithLogger(logger *slog.Logger) Option {
	return func(c *config) {
		c.logger = logger
	}
}
