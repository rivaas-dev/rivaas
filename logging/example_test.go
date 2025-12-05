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

package logging_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"rivaas.dev/logging"
)

// ExampleNew demonstrates creating a new logger with basic configuration.
// The logger outputs JSON-formatted logs to stdout.
func ExampleNew() {
	logger, err := logging.New(
		logging.WithJSONHandler(),
		logging.WithServiceName("my-service"),
		logging.WithServiceVersion("1.0.0"),
	)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer logger.Shutdown(context.Background())

	logger.Info("service started", "port", 8080)
	// Output is non-deterministic (contains timestamps)
}

// ExampleNew_validation demonstrates that New validates configuration.
func ExampleNew_validation() {
	_, err := logging.New(logging.WithOutput(nil))
	if err != nil {
		fmt.Println("validation error:", err != nil)
	}
	// Output: validation error: true
}

// ExampleMustNew demonstrates creating a logger that panics on error.
// This is useful for application initialization where errors are fatal.
func ExampleMustNew() {
	logger := logging.MustNew(
		logging.WithConsoleHandler(),
		logging.WithDebugLevel(),
	)
	defer logger.Shutdown(context.Background())

	logger.Info("application initialized")
	logger.Debug("debug information", "key", "value")
	// Output is non-deterministic (contains timestamps and colors)
}

// ExampleLogger_ServiceName demonstrates accessing logger configuration.
func ExampleLogger_ServiceName() {
	logger := logging.MustNew(
		logging.WithJSONHandler(),
		logging.WithOutput(io.Discard),
		logging.WithServiceName("my-api"),
		logging.WithServiceVersion("2.0.0"),
		logging.WithEnvironment("production"),
	)
	defer logger.Shutdown(context.Background())

	fmt.Printf("service: %s\n", logger.ServiceName())
	fmt.Printf("version: %s\n", logger.ServiceVersion())
	fmt.Printf("env: %s\n", logger.Environment())
	// Output:
	// service: my-api
	// version: 2.0.0
	// env: production
}

// ExampleLogger_Level demonstrates accessing and checking the log level.
func ExampleLogger_Level() {
	logger := logging.MustNew(
		logging.WithJSONHandler(),
		logging.WithOutput(io.Discard),
		logging.WithLevel(logging.LevelWarn),
	)
	defer logger.Shutdown(context.Background())

	level := logger.Level()
	fmt.Printf("level: %s\n", level.String())
	// Output: level: WARN
}

// ExampleLogger_SetLevel demonstrates dynamic level changes at runtime.
func ExampleLogger_SetLevel() {
	logger := logging.MustNew(
		logging.WithJSONHandler(),
		logging.WithOutput(io.Discard),
		logging.WithLevel(logging.LevelInfo),
	)
	defer logger.Shutdown(context.Background())

	fmt.Printf("initial: %s\n", logger.Level().String())

	err := logger.SetLevel(logging.LevelDebug)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Printf("changed: %s\n", logger.Level().String())
	// Output:
	// initial: INFO
	// changed: DEBUG
}

// ExampleLogger_IsEnabled demonstrates checking if logging is enabled.
func ExampleLogger_IsEnabled() {
	logger := logging.MustNew(
		logging.WithJSONHandler(),
		logging.WithOutput(io.Discard),
	)

	fmt.Printf("before shutdown: %v\n", logger.IsEnabled())

	logger.Shutdown(context.Background())

	fmt.Printf("after shutdown: %v\n", logger.IsEnabled())
	// Output:
	// before shutdown: true
	// after shutdown: false
}

// ExampleLogger_LogRequest demonstrates logging HTTP requests.
// This helper method automatically extracts common request fields.
func ExampleLogger_LogRequest() {
	logger := logging.MustNew(logging.WithJSONHandler())
	defer logger.Shutdown(context.Background())

	// Simulate an HTTP request
	req, _ := http.NewRequest("GET", "/api/users?page=1", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	req.Header.Set("User-Agent", "MyApp/1.0")

	logger.LogRequest(req, "status", 200, "duration_ms", 45)
	// Output is non-deterministic (contains timestamps)
}

//nolint:testableexamples // Output is non-deterministic (contains timestamps and colors)

// ExampleLogger_LogError demonstrates logging errors with context.
// The error message is automatically added as the "error" attribute.
func ExampleLogger_LogError() {
	logger := logging.MustNew(logging.WithConsoleHandler())
	defer logger.Shutdown(context.Background())

	err := fmt.Errorf("database connection failed")
	logger.LogError(err, "database operation failed",
		"operation", "INSERT",
		"table", "users",
		"retry_count", 3,
	)
	// Output is non-deterministic (contains timestamps and colors)
}

// ExampleLogger_LogDuration demonstrates logging operation duration.
// Both human-readable duration and milliseconds are automatically included.
func ExampleLogger_LogDuration() {
	logger := logging.MustNew(logging.WithJSONHandler())
	defer logger.Shutdown(context.Background())

	start := time.Now()
	// Simulate some work
	time.Sleep(10 * time.Millisecond)

	logger.LogDuration("data processing completed", start,
		"rows_processed", 1000,
		"errors", 0,
	)
	// Output is non-deterministic (contains timestamps and duration)
}

//nolint:testableexamples // Output is non-deterministic (contains timestamps and stack traces)

// ExampleLogger_ErrorWithStack demonstrates error logging with stack traces.
// Stack traces should only be enabled for critical errors to avoid performance overhead.
func ExampleLogger_ErrorWithStack() {
	logger := logging.MustNew(logging.WithConsoleHandler())
	defer logger.Shutdown(context.Background())

	err := fmt.Errorf("critical error occurred")
	logger.ErrorWithStack("critical error", err, true,
		"component", "payment-processor",
		"transaction_id", "tx-12345",
	)
}

// ExampleWithSampling demonstrates log sampling for high-traffic scenarios.
// Sampling reduces log volume while maintaining visibility.
func ExampleWithSampling() {
	logger := logging.MustNew(
		logging.WithJSONHandler(),
		logging.WithSampling(logging.SamplingConfig{
			Initial:    100,         // Log first 100 entries unconditionally
			Thereafter: 100,         // Then log 1 in every 100
			Tick:       time.Minute, // Reset counter every minute
		}),
	)
	defer logger.Shutdown(context.Background())

	// In high-traffic scenarios, this will sample logs
	for i := 0; i < 500; i++ {
		logger.Info("request processed", "request_id", i)
	}
	// Output is non-deterministic (sampling behavior varies)
}

// ExampleWithReplaceAttr demonstrates custom attribute replacement.
// This is useful for custom redaction rules beyond the built-in sensitive fields.
func ExampleWithReplaceAttr() {
	logger := logging.MustNew(
		logging.WithJSONHandler(),
		logging.WithReplaceAttr(func(groups []string, a slog.Attr) slog.Attr {
			// Redact sensitive fields
			if a.Key == "password" || a.Key == "token" {
				return slog.String(a.Key, "***REDACTED***")
			}
			return a
		}),
	)
	defer logger.Shutdown(context.Background())

	logger.Info("user login", "username", "alice", "password", "secret123")
	// Output is non-deterministic (contains timestamps)
}

// ExampleNewTestLogger demonstrates creating a logger for testing.
func ExampleNewTestLogger() {
	logger, buf := logging.NewTestLogger()
	defer logger.Shutdown(context.Background())

	logger.Info("test message", "key", "value")

	entries, _ := logging.ParseJSONLogEntries(buf)
	fmt.Printf("logged %d entries\n", len(entries))
	fmt.Printf("message: %s\n", entries[0].Message)
	// Output:
	// logged 1 entries
	// message: test message
}

// ExampleParseJSONLogEntries demonstrates parsing JSON log entries from a buffer.
func ExampleParseJSONLogEntries() {
	buf := &bytes.Buffer{}
	logger := logging.MustNew(
		logging.WithJSONHandler(),
		logging.WithOutput(buf),
	)
	defer logger.Shutdown(context.Background())

	logger.Info("first message")
	logger.Warn("second message", "count", 42)

	entries, err := logging.ParseJSONLogEntries(buf)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Printf("entry count: %d\n", len(entries))
	fmt.Printf("first level: %s\n", entries[0].Level)
	fmt.Printf("second level: %s\n", entries[1].Level)
	// Output:
	// entry count: 2
	// first level: INFO
	// second level: WARN
}

// ExampleNewBatchLogger demonstrates batched logging for performance.
func ExampleNewBatchLogger() {
	logger := logging.MustNew(
		logging.WithJSONHandler(),
		logging.WithOutput(io.Discard),
	)
	defer logger.Shutdown(context.Background())

	// Create batch logger that flushes every 100 entries or 1 second
	bl := logging.NewBatchLogger(logger, 100, time.Second)
	defer bl.Close()

	// Add log entries to the batch
	bl.Info("first event")
	bl.Info("second event")

	fmt.Printf("batch size: %d\n", bl.Size())

	// Manual flush writes all pending entries
	bl.Flush()

	fmt.Printf("after flush: %d\n", bl.Size())
	// Output:
	// batch size: 2
	// after flush: 0
}

// ExampleNewContextLogger demonstrates context-aware logging.
func ExampleNewContextLogger() {
	buf := &bytes.Buffer{}
	logger := logging.MustNew(
		logging.WithJSONHandler(),
		logging.WithOutput(buf),
	)
	defer logger.Shutdown(context.Background())

	// Create a context logger and add request-scoped fields via With()
	ctx := context.Background()
	cl := logging.NewContextLogger(ctx, logger)

	// Use With() to add fields to the underlying logger
	clWithFields := cl.With("request_id", "req-123", "user_id", "user-456")
	clWithFields.Info("processing request")

	entries, _ := logging.ParseJSONLogEntries(buf)
	fmt.Printf("request_id: %s\n", entries[0].Attrs["request_id"])
	fmt.Printf("user_id: %s\n", entries[0].Attrs["user_id"])
	// Output:
	// request_id: req-123
	// user_id: user-456
}
