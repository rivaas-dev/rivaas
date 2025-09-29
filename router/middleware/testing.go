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

package middleware

import (
	"io"
	"log/slog"
	"os"
)

// NewTestLogger creates a silent logger for tests.
// This logger discards all output, making tests clean and focused.
//
// Example:
//
//	import "rivaas.dev/router/middleware"
//
//	func TestAccessLog(t *testing.T) {
//	    logger := middleware.NewTestLogger()
//	    r := router.New()
//	    r.Use(accesslog.New(accesslog.WithLogger(logger)))
//	    // ... test code
//	}
func NewTestLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

// NewCaptureLogger creates a logger that writes to the provided writer.
// This is useful for testing when you need to verify log output.
//
// Example:
//
//	import (
//	    "bytes"
//	    "testing"
//	    "rivaas.dev/router/middleware"
//	)
//
//	func TestAccessLogOutput(t *testing.T) {
//	    var buf bytes.Buffer
//	    logger := middleware.NewCaptureLogger(&buf)
//	    r := router.New()
//	    r.Use(accesslog.New(accesslog.WithLogger(logger)))
//	    // ... test code
//	    // Verify log output
//	    if !strings.Contains(buf.String(), "expected log message") {
//	        t.Error("expected log message not found")
//	    }
//	}
func NewCaptureLogger(w io.Writer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, nil))
}

// NewDebugLogger creates a human-readable text logger for debugging tests.
// Output goes to stdout with DEBUG level enabled.
//
// Example:
//
//	import "rivaas.dev/router/middleware"
//
//	func TestAccessLogDebug(t *testing.T) {
//	    if testing.Verbose() {
//	        logger := middleware.NewDebugLogger()
//	        r := router.New()
//	        r.Use(accesslog.New(accesslog.WithLogger(logger)))
//	    }
//	    // ... test code
//	}
func NewDebugLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}
