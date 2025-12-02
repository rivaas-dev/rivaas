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

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"
)

// logAttrPool provides pooled attribute slices for convenience methods.
// [Logger.LogRequest], [Logger.LogError], and [Logger.LogDuration] use this pool
// to build attribute lists.
var logAttrPool = sync.Pool{
	New: func() any {
		s := make([]any, 0, 16)
		return &s
	},
}

// LogRequest logs an HTTP request with standard fields.
//
// Standard fields included:
//   - method: HTTP method (GET, POST, etc.)
//   - path: Request path (without query string)
//   - remote: Client remote address
//   - user_agent: Client User-Agent header
//   - query: Query string (only if non-empty)
//
// Additional fields can be passed via 'extra' (e.g., "status", 200, "duration_ms", 45).
//
// Thread-safe and safe to call concurrently.
//
// Example:
//
//	logger.LogRequest(r, "status", 200, "duration_ms", 45, "bytes", 1024)
func (l *Logger) LogRequest(r *http.Request, extra ...any) {
	if l.isShuttingDown.Load() {
		return
	}

	attrsPtr := logAttrPool.Get().(*[]any)
	attrs := (*attrsPtr)[:0]
	defer func() {
		*attrsPtr = (*attrsPtr)[:0]
		logAttrPool.Put(attrsPtr)
	}()

	attrs = append(attrs,
		"method", r.Method,
		"path", r.URL.Path,
		"remote", r.RemoteAddr,
		"user_agent", r.UserAgent(),
	)
	if r.URL.RawQuery != "" {
		attrs = append(attrs, "query", r.URL.RawQuery)
	}
	attrs = append(attrs, extra...)
	l.Info("http request", attrs...)
}

// LogError logs an error with additional context fields.
//
// Why use this instead of Error():
//   - Automatically includes "error" field with error message
//   - Convenient for error handling patterns
//   - Consistent error logging format across codebase
//
// Thread-safe and safe to call concurrently.
//
// Example:
//
//	if err := db.Insert(user); err != nil {
//	    logger.LogError(err, "database operation failed",
//	        "operation", "INSERT",
//	        "table", "users",
//	        "retry_count", 3,
//	    )
//	    return err
//	}
func (l *Logger) LogError(err error, msg string, extra ...any) {
	if l.isShuttingDown.Load() {
		return
	}

	attrsPtr := logAttrPool.Get().(*[]any)
	attrs := (*attrsPtr)[:0]
	defer func() {
		*attrsPtr = (*attrsPtr)[:0]
		logAttrPool.Put(attrsPtr)
	}()

	attrs = append(attrs, "error", err.Error())
	attrs = append(attrs, extra...)
	l.Error(msg, attrs...)
}

// LogDuration logs an operation duration with timing information.
//
// Automatically includes:
//   - duration_ms: Duration in milliseconds (for easy filtering/alerting)
//   - duration: Human-readable duration string (e.g., "1.5s", "250ms")
//
// Thread-safe and safe to call concurrently.
//
// Example:
//
//	start := time.Now()
//	result, err := processData(data)
//	logger.LogDuration("data processing completed", start,
//	    "rows_processed", result.Count,
//	    "errors", result.Errors,
//	)
func (l *Logger) LogDuration(msg string, start time.Time, extra ...any) {
	if l.isShuttingDown.Load() {
		return
	}

	duration := time.Since(start)
	attrsPtr := logAttrPool.Get().(*[]any)
	attrs := (*attrsPtr)[:0]
	defer func() {
		*attrsPtr = (*attrsPtr)[:0]
		logAttrPool.Put(attrsPtr)
	}()

	attrs = append(attrs,
		"duration_ms", duration.Milliseconds(),
		"duration", duration.String(),
	)
	attrs = append(attrs, extra...)
	l.Info(msg, attrs...)
}

// ErrorWithStack logs an error with optional stack trace.
//
// When to use stack traces:
//
//	✓ Critical errors that require debugging
//	✓ Unexpected error conditions (panics, invariant violations)
//	✗ Expected errors (validation failures, not found)
//	✗ High-frequency errors where stack capture cost is undesirable
//
// Thread-safe and safe to call concurrently.
func (l *Logger) ErrorWithStack(msg string, err error, includeStack bool, extra ...any) {
	if l.isShuttingDown.Load() {
		return
	}

	attrsPtr := logAttrPool.Get().(*[]any)
	attrs := (*attrsPtr)[:0]
	defer func() {
		*attrsPtr = (*attrsPtr)[:0]
		logAttrPool.Put(attrsPtr)
	}()

	attrs = append(attrs, "error", err.Error())

	if includeStack {
		attrs = append(attrs, "stack", captureStack(3))
	}

	attrs = append(attrs, extra...)

	l.log(slog.LevelError, msg, attrs...)
}

// captureStack captures a stack trace.
//
// Skip parameter: Number of stack frames to skip.
//   - 0: includes captureStack itself
//   - 3: typical value to skip captureStack, ErrorWithStack, and caller's caller
func captureStack(skip int) string {
	var buf strings.Builder
	pcs := make([]uintptr, 10)
	n := runtime.Callers(skip, pcs)
	frames := runtime.CallersFrames(pcs[:n])

	for {
		frame, more := frames.Next()
		fmt.Fprintf(&buf, "%s\n\t%s:%d\n", frame.Function, frame.File, frame.Line)
		if !more {
			break
		}
	}
	return buf.String()
}
