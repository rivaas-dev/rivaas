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
// Basic usage:
//
//	logger := logging.MustNew(logging.WithConsoleHandler())
//	defer logger.Shutdown(context.Background())
//	logger.Info("service started", "port", 8080)
//
// With structured logging:
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
// To register as the global slog default (for use with slog.Info(), etc.):
//
//	logger := logging.MustNew(
//	    logging.WithJSONHandler(),
//	    logging.WithGlobalLogger(), // Sets slog.SetDefault()
//	)
//
// Sensitive data (password, token, secret, api_key, authorization) is
// automatically redacted from all log output. Additional sanitization can be
// configured using WithReplaceAttr.
//
// See the README for more examples and configuration options.
package logging
