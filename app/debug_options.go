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

package app

// DebugOption configures debug endpoint settings.
// These options configure pprof and other debug endpoints.
type DebugOption func(*debugSettings)

// debugSettings holds debug endpoint configuration.
type debugSettings struct {
	enabled bool

	// Path configuration
	prefix string // Mount prefix (default: "/debug")

	// Feature toggles
	pprofEnabled bool // Enable pprof endpoints
}

// defaultDebugSettings returns debug settings with sensible defaults.
func defaultDebugSettings() *debugSettings {
	return &debugSettings{
		enabled: true, // Enabled by default when WithDebugEndpoints is called
		prefix:  "/debug",
	}
}

// WithDebugPrefix sets the mount prefix for debug endpoints.
// Default is "/debug", which mounts pprof at "/debug/pprof/*".
//
// Example:
//
//	app.MustNew(
//	    app.WithDebugEndpoints(
//	        app.WithDebugPrefix("/_debug"),
//	        app.WithPprof(),
//	    ),
//	)
//	// Endpoints: /_debug/pprof/*, etc.
func WithDebugPrefix(prefix string) DebugOption {
	return func(s *debugSettings) {
		s.prefix = prefix
	}
}

// WithPprof enables pprof endpoints for profiling and debugging.
//
// Security rationale: pprof endpoints are disabled by default and require explicit
// opt-in because they expose sensitive runtime information that can be exploited:
//
// Attack vectors:
//   - Goroutine dumps reveal internal logic and potential race conditions
//   - Heap dumps may contain secrets, tokens, or PII in memory
//   - CPU profiling can be used for timing attacks or DoS (profiling has overhead)
//   - Alloc profiles reveal memory usage patterns useful for resource exhaustion attacks
//
// Safe usage patterns:
//  1. Development: Enable unconditionally (no external exposure)
//  2. Staging: Enable behind VPN or IP allowlist
//  3. Production: Enable only with proper authentication middleware
//     Example: app.Use(authMiddleware); app.WithDebugEndpoints(app.WithPprof())
//
// Endpoints registered (when enabled):
//   - GET /debug/pprof/ - Main pprof index
//   - GET /debug/pprof/cmdline - Command line
//   - GET /debug/pprof/profile - CPU profile
//   - GET /debug/pprof/symbol - Symbol lookup
//   - POST /debug/pprof/symbol - Symbol lookup
//   - GET /debug/pprof/trace - Execution trace
//   - GET /debug/pprof/{profile} - Named profiles (allocs, block, goroutine, heap, mutex, threadcreate)
//
// Example:
//
//	// Development: enable pprof
//	app.MustNew(
//	    app.WithDebugEndpoints(
//	        app.WithPprof(),
//	    ),
//	)
//
//	// Production: enable only if explicitly requested via environment
//	app.MustNew(
//	    app.WithDebugEndpoints(
//	        app.WithPprofIf(os.Getenv("PPROF_ENABLED") == "true"),
//	    ),
//	)
func WithPprof() DebugOption {
	return func(s *debugSettings) {
		s.pprofEnabled = true
	}
}

// WithPprofIf conditionally enables pprof endpoints based on the given condition.
// This is useful for environment-based configuration.
//
// Example:
//
//	app.MustNew(
//	    app.WithDebugEndpoints(
//	        app.WithPprofIf(os.Getenv("PPROF_ENABLED") == "true"),
//	    ),
//	)
func WithPprofIf(condition bool) DebugOption {
	return func(s *debugSettings) {
		s.pprofEnabled = condition
	}
}

// WithDebugEndpoints enables and configures debug endpoints.
// By default, no debug features are enabled - you must explicitly opt-in
// to specific features like pprof for security reasons.
//
// Example:
//
//	// Development: full debug access
//	app.MustNew(
//	    app.WithServiceName("orders-api"),
//	    app.WithDebugEndpoints(
//	        app.WithPprof(),
//	    ),
//	)
//
//	// Production: conditional debug access
//	app.MustNew(
//	    app.WithServiceName("orders-api"),
//	    app.WithDebugEndpoints(
//	        app.WithDebugPrefix("/_internal/debug"),
//	        app.WithPprofIf(os.Getenv("ENABLE_DEBUG") == "true"),
//	    ),
//	)
func WithDebugEndpoints(opts ...DebugOption) Option {
	return func(c *config) {
		if c.debug == nil {
			c.debug = defaultDebugSettings()
		}
		for _, opt := range opts {
			opt(c.debug)
		}
	}
}
