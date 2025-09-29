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
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"rivaas.dev/logging"
)

// ExampleNew demonstrates creating a new logger with basic configuration.
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
}

// ExampleMustNew demonstrates creating a logger that panics on error.
func ExampleMustNew() {
	logger := logging.MustNew(
		logging.WithConsoleHandler(),
		logging.WithDebugLevel(),
	)
	defer logger.Shutdown(context.Background())

	logger.Info("application initialized")
	logger.Debug("debug information", "key", "value")
}

// ExampleLogRequest demonstrates logging HTTP requests.
func ExampleConfig_LogRequest() {
	logger := logging.MustNew(logging.WithJSONHandler())
	defer logger.Shutdown(context.Background())

	// Simulate an HTTP request
	req, _ := http.NewRequest("GET", "/api/users?page=1", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	req.Header.Set("User-Agent", "MyApp/1.0")

	logger.LogRequest(req, "status", 200, "duration_ms", 45)
}

// ExampleLogError demonstrates logging errors with context.
func ExampleConfig_LogError() {
	logger := logging.MustNew(logging.WithConsoleHandler())
	defer logger.Shutdown(context.Background())

	err := fmt.Errorf("database connection failed")
	logger.LogError(err, "database operation failed",
		"operation", "INSERT",
		"table", "users",
		"retry_count", 3,
	)
}

// ExampleLogDuration demonstrates logging operation duration.
func ExampleConfig_LogDuration() {
	logger := logging.MustNew(logging.WithJSONHandler())
	defer logger.Shutdown(context.Background())

	start := time.Now()
	// Simulate some work
	time.Sleep(10 * time.Millisecond)

	logger.LogDuration("data processing completed", start,
		"rows_processed", 1000,
		"errors", 0,
	)
}

// ExampleErrorWithStack demonstrates error logging with stack traces.
func ExampleConfig_ErrorWithStack() {
	logger := logging.MustNew(logging.WithConsoleHandler())
	defer logger.Shutdown(context.Background())

	err := fmt.Errorf("critical error occurred")
	logger.ErrorWithStack("critical error", err, true,
		"component", "payment-processor",
		"transaction_id", "tx-12345",
	)
}

// ExampleWithSampling demonstrates log sampling for high-traffic scenarios.
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
}

// ExampleWithReplaceAttr demonstrates custom attribute replacement.
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
}
