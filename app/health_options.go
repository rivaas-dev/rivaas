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

import (
	"context"
	"time"
)

// HealthOption configures health endpoint settings.
// These options configure liveness (/healthz) and readiness (/readyz) probes.
type HealthOption func(*healthSettings)

// CheckFunc defines a function that performs a health or readiness check.
// The function should return nil if the check passes, or an error if it fails.
// The context may be cancelled if the check takes too long.
type CheckFunc func(ctx context.Context) error

// healthSettings holds health endpoint configuration.
type healthSettings struct {
	enabled bool

	// Path configuration
	prefix      string // Mount prefix (e.g., "/_system")
	healthzPath string // Liveness probe path (default: "/healthz")
	readyzPath  string // Readiness probe path (default: "/readyz")

	// Check configuration
	liveness  map[string]CheckFunc // Liveness checks
	readiness map[string]CheckFunc // Readiness checks
	timeout   time.Duration        // Timeout for each check
}

// defaultHealthSettings returns health settings with sensible defaults.
func defaultHealthSettings() *healthSettings {
	return &healthSettings{
		enabled:     true, // Enabled by default when WithHealthEndpoints is called
		healthzPath: "/healthz",
		readyzPath:  "/readyz",
		timeout:     time.Second,
		liveness:    make(map[string]CheckFunc),
		readiness:   make(map[string]CheckFunc),
	}
}

// WithHealthPrefix sets the mount prefix for health endpoints.
// By default, endpoints are mounted at root (e.g., /healthz, /readyz).
// Use this to mount under a different prefix (e.g., /_system/healthz).
//
// Example:
//
//	app.MustNew(
//	    app.WithHealthEndpoints(
//	        app.WithHealthPrefix("/_system"),
//	    ),
//	)
//	// Endpoints: /_system/healthz, /_system/readyz
func WithHealthPrefix(prefix string) HealthOption {
	return func(s *healthSettings) {
		s.prefix = prefix
	}
}

// WithHealthzPath sets the path for the liveness probe endpoint.
// Default is "/healthz". The path is appended to the prefix (if set).
//
// Example:
//
//	app.WithHealthEndpoints(
//	    app.WithHealthzPath("/live"),
//	)
//	// Endpoint: /live (or /{prefix}/live if prefix is set)
func WithHealthzPath(path string) HealthOption {
	return func(s *healthSettings) {
		s.healthzPath = path
	}
}

// WithReadyzPath sets the path for the readiness probe endpoint.
// Default is "/readyz". The path is appended to the prefix (if set).
//
// Example:
//
//	app.WithHealthEndpoints(
//	    app.WithReadyzPath("/ready"),
//	)
//	// Endpoint: /ready (or /{prefix}/ready if prefix is set)
func WithReadyzPath(path string) HealthOption {
	return func(s *healthSettings) {
		s.readyzPath = path
	}
}

// WithHealthTimeout sets the timeout for each health check.
// Each check runs with an independent context.WithTimeout to prevent
// one slow dependency from blocking the entire health check.
// Default is 1 second.
//
// Example:
//
//	app.WithHealthEndpoints(
//	    app.WithHealthTimeout(500 * time.Millisecond),
//	)
func WithHealthTimeout(d time.Duration) HealthOption {
	return func(s *healthSettings) {
		s.timeout = d
	}
}

// WithLivenessCheck adds a liveness check.
// Liveness checks determine if the process is alive and should be dependency-free.
// If any liveness check fails, /healthz returns 503.
//
// Multiple calls accumulate checks. If no liveness checks are provided,
// /healthz always returns 200 (process is running).
//
// Example:
//
//	app.WithHealthEndpoints(
//	    app.WithLivenessCheck("process", func(ctx context.Context) error {
//	        // Dependency-free check: process is alive
//	        return nil
//	    }),
//	    app.WithLivenessCheck("goroutines", func(ctx context.Context) error {
//	        if runtime.NumGoroutine() > 10000 {
//	            return errors.New("too many goroutines")
//	        }
//	        return nil
//	    }),
//	)
func WithLivenessCheck(name string, check CheckFunc) HealthOption {
	return func(s *healthSettings) {
		if s.liveness == nil {
			s.liveness = make(map[string]CheckFunc)
		}
		s.liveness[name] = check
	}
}

// WithReadinessCheck adds a readiness check.
// Readiness checks determine if the service is ready to accept traffic.
// These typically check external dependencies (database, cache, external APIs).
// If any readiness check fails, /readyz returns 503.
//
// Multiple calls accumulate checks. If no readiness checks are provided,
// /readyz always returns 204 (service is ready).
//
// Example:
//
//	app.WithHealthEndpoints(
//	    app.WithReadinessCheck("database", func(ctx context.Context) error {
//	        return db.PingContext(ctx)
//	    }),
//	    app.WithReadinessCheck("cache", func(ctx context.Context) error {
//	        return redis.Ping(ctx).Err()
//	    }),
//	    app.WithReadinessCheck("upstream", func(ctx context.Context) error {
//	        resp, err := http.Get("https://api.example.com/health")
//	        if err != nil {
//	            return err
//	        }
//	        defer resp.Body.Close()
//	        if resp.StatusCode != http.StatusOK {
//	            return errors.New("upstream unhealthy")
//	        }
//	        return nil
//	    }),
//	)
func WithReadinessCheck(name string, check CheckFunc) HealthOption {
	return func(s *healthSettings) {
		if s.readiness == nil {
			s.readiness = make(map[string]CheckFunc)
		}
		s.readiness[name] = check
	}
}

// WithHealthEndpoints enables and configures health check endpoints.
// This registers /healthz (liveness) and /readyz (readiness) endpoints.
//
// Endpoints registered:
//   - GET /healthz (or /{prefix}/healthz) - Liveness probe
//     Returns 200 "ok" if all liveness checks pass, 503 if any fail
//   - GET /readyz (or /{prefix}/readyz) - Readiness probe
//     Returns 204 if all readiness checks pass, 503 if any fail
//
// Example:
//
//	app.MustNew(
//	    app.WithServiceName("orders-api"),
//	    app.WithHealthEndpoints(
//	        app.WithHealthPrefix("/_system"),
//	        app.WithHealthTimeout(800 * time.Millisecond),
//	        app.WithLivenessCheck("process", func(ctx context.Context) error {
//	            return nil // Process is alive
//	        }),
//	        app.WithReadinessCheck("database", func(ctx context.Context) error {
//	            return db.PingContext(ctx)
//	        }),
//	        app.WithReadinessCheck("cache", func(ctx context.Context) error {
//	            return redis.Ping(ctx).Err()
//	        }),
//	    ),
//	)
func WithHealthEndpoints(opts ...HealthOption) Option {
	return func(c *config) {
		if c.health == nil {
			c.health = defaultHealthSettings()
		}
		for _, opt := range opts {
			opt(c.health)
		}
	}
}
