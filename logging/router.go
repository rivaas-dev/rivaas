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

package logging

import "log/slog"

// Recorder is the interface used by router/app packages to access logging.
//
// Design rationale: The provider interface exists to support diverse operational
// requirements without coupling application code to specific logging systems:
//
// 1. Development: Use slog with pretty-printed console output
// 2. Testing: Use in-memory or no-op providers for clean tests
// 3. Production: Use vendor-specific SDKs (Datadog, New Relic) with batching
// 4. Migration: Switch providers without changing application logging calls
//
// Alternative considered: Direct slog usage would eliminate abstraction but
// couples code to stdlib, making vendor SDK integration difficult.
//
// Design pattern: Dependency inversion principle - router depends on
// abstraction (Recorder) not concrete implementation (Config).
type Recorder interface {
	Logger() *slog.Logger
	With(args ...any) *slog.Logger
	WithGroup(name string) *slog.Logger
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// RouterOption mirrors metrics/tracing pattern to avoid import cycles.
//
// Why interface{} parameter:
//   - Allows type-safe dynamic dispatch without generics
//   - Router implementation can be any type with SetLogger method
//   - Avoids circular dependency between packages
//
// This is a common pattern in the Rivaas package architecture for
// composable middleware configuration.
type RouterOption func(interface{})

// WithLogging enables logging on a router with the specified options.
//
// Pattern: Functional options for composable configuration.
//
// The router must implement SetLogger(Logger) interface. This is checked
// at runtime using type assertion. If the router doesn't implement the
// interface, the option silently does nothing (fail-safe behavior).
//
// Example usage:
//
//	r := router.MustNew(
//	    logging.WithLogging(
//	        logging.WithConsoleHandler(),  // Development-friendly output
//	        logging.WithDebugLevel(),       // Verbose logging
//	        logging.WithServiceName("api"), // Service identification
//	    ),
//	)
//
// This creates and configures a new logger specifically for the router.
func WithLogging(opts ...Option) RouterOption {
	return func(router interface{}) {
		cfg := MustNew(opts...)
		if setter, ok := router.(interface{ SetLogger(Logger) }); ok {
			setter.SetLogger(cfg)
		}
	}
}

// WithLoggingFromConfig wires an existing logger into the router.
//
// When to use this vs WithLogging:
//   - WithLogging: Creates new logger instance (typical case)
//   - WithLoggingFromConfig: Shares existing logger across components
//
// Use case for sharing:
//   - Multiple routers that should use the same logger
//   - Router and background jobs sharing configuration
//   - Testing with pre-configured mock logger
//
// Example:
//
//	// Shared logger for multiple components
//	logger := logging.MustNew(
//	    logging.WithJSONHandler(),
//	    logging.WithServiceName("api"),
//	)
//
//	router1 := router.MustNew(logging.WithLoggingFromConfig(logger))
//	router2 := router.MustNew(logging.WithLoggingFromConfig(logger))
//
//	// Both routers use the same logger instance
func WithLoggingFromConfig(cfg *Config) RouterOption {
	return func(router interface{}) {
		if setter, ok := router.(interface{ SetLogger(Logger) }); ok {
			setter.SetLogger(cfg)
		}
	}
}
