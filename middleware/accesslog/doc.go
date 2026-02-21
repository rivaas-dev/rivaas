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

// Package accesslog provides middleware for structured HTTP access logging
// with configurable formats, field selection, and output destinations.
//
// This middleware logs HTTP requests and responses with detailed information
// including method, path, status code, response time, client IP, user agent,
// and custom fields. It supports multiple output formats (JSON, Common Log Format,
// Combined Log Format) and can filter or exclude specific paths.
//
// # Basic Usage
//
//	import (
//	    "log/slog"
//	    "os"
//	    "rivaas.dev/middleware/accesslog"
//	)
//
//	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
//	r := router.MustNew()
//	r.Use(accesslog.New(
//	    accesslog.WithLogger(logger),
//	))
//
// # Configuration Options
//
//   - Logger: Structured logger (slog.Logger) for output
//   - Format: Output format (JSON, CommonLog, CombinedLog)
//   - ExcludePaths: Paths to exclude from logging (e.g., /health, /metrics)
//   - Fields: Custom fields to include in logs
//   - Sampling: Rate-based sampling to reduce log volume
//   - IPAnonymization: Anonymize IP addresses for privacy compliance
//
// # Log Fields
//
// The middleware logs comprehensive request/response information:
//
//   - Method, Path, Status: HTTP request/response details
//   - Duration: Request processing time
//   - ClientIP: Real client IP (handles proxies)
//   - UserAgent: Client user agent string
//   - RequestID: Correlation ID from requestid middleware
//   - Custom fields: User-defined additional fields
package accesslog
