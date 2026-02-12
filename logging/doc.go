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

// Package logging provides structured logging with multiple provider backends.
//
// Design philosophy: This package abstracts logging providers to enable:
//   - Zero-dependency default (slog in stdlib)
//   - Drop-in replacements for existing logging infrastructure
//   - Testing with in-memory or no-op providers
//
// The abstraction enables operational flexibility: production systems can switch
// providers without code changes (e.g., migrating from ELK to Datadog, or adding
// sampling in high-traffic systems).
//
// # Basic Usage
//
//	logger := logging.MustNew(logging.WithConsoleHandler())
//	defer logger.Shutdown(context.Background())
//	logger.Info("service started", "port", 8080)
//
// # Structured Logging
//
//	logger := logging.MustNew(
//	    logging.WithJSONHandler(),
//	    logging.WithServiceName("my-service"),
//	    logging.WithDebugLevel(),
//	)
//	defer logger.Shutdown(context.Background())
//	logger.Info("request processed",
//	    "method", http.MethodGet,
//	    "path", "/api/users",
//	    "status", 200,
//	)
//
// # Convenience Methods
//
// The package provides helper methods for common logging patterns:
//
//	// HTTP request logging
//	logger.LogRequest(r, "status", 200, "duration_ms", 45)
//
//	// Error logging with context
//	logger.LogError(err, "operation failed", "user_id", userID)
//
//	// Duration tracking
//	start := time.Now()
//	logger.LogDuration("processing completed", start, "items", count)
//
// # Log Sampling
//
// Reduce log volume in high-traffic scenarios:
//
//	logger := logging.MustNew(
//	    logging.WithJSONHandler(),
//	    logging.WithSampling(logging.SamplingConfig{
//	        Initial:    100,          // Log first 100 entries
//	        Thereafter: 100,          // Then log 1 in 100
//	        Tick:       time.Minute,  // Reset every minute
//	    }),
//	)
//
// Note: Errors (level >= ERROR) always bypass sampling.
//
// # Dynamic Log Levels
//
// Change log levels at runtime:
//
//	logger.SetLevel(logging.LevelDebug)  // Enable debug logging
//	logger.SetLevel(logging.LevelWarn)   // Reduce to warnings only
//
// # Global Logger Registration
//
// To register as the global slog default (for use with slog.Info(), etc.):
//
//	logger := logging.MustNew(
//	    logging.WithJSONHandler(),
//	    logging.WithGlobalLogger(), // Sets slog.SetDefault()
//	)
//
// By default, loggers are NOT registered globally to allow multiple independent
// logger instances in the same process.
//
// # Sensitive Data Redaction
//
// Sensitive data (password, token, secret, api_key, authorization) is
// automatically redacted from all log output. Additional sanitization can be
// configured using WithReplaceAttr.
//
// # Context-Aware Logging
//
// Trace correlation with OpenTelemetry is automatic. When using
// slog.*Context methods with a context that contains an active OTel span,
// trace_id and span_id are injected into every log record:
//
//	slog.InfoContext(ctx, "processing request", "user_id", userID)
//	// Automatically includes trace_id and span_id if context has active span
//
// See the README for more examples and configuration options.
package logging
