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

// Package main demonstrates logging helper methods and utilities.
//
// This example covers:
//   - LogDuration for timing operations
//   - LogError and ErrorWithStack for error handling
//   - LogRequest for HTTP request logging
//   - Context-based field injection for request-scoped loggers
//   - DebugInfo for diagnostic information
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"rivaas.dev/logging"
)

func main() {
	logger := logging.MustNew(
		logging.WithConsoleHandler(),
		logging.WithLevel(logging.LevelInfo),
		logging.WithSource(true),
	)

	// 1. Duration logging - measure operation timing
	demonstrateDurationLogging(logger)

	// 2. Error logging with and without stack traces
	demonstrateErrorLogging(logger)

	// 3. HTTP request logging
	demonstrateRequestLogging(logger)

	// 4. Context-based request-scoped logging
	demonstrateContextLogging(logger)

	// 5. Debug info for diagnostics
	demonstrateDebugInfo(logger)
}

func demonstrateDurationLogging(logger *logging.Logger) {
	fmt.Println("\n--- Duration Logging ---")

	// Success path: log operation timing
	start := time.Now()
	processOrder("order-123")
	logger.LogDuration("order processed", start,
		"order_id", "order-123",
		"status", "completed",
	)

	// Error path: log timing even on failure
	start = time.Now()
	if err := processOrder("order-invalid"); err != nil {
		logger.LogDuration("order processing failed", start,
			"order_id", "order-invalid",
			"error", err.Error(),
		)
	}
}

func demonstrateErrorLogging(logger *logging.Logger) {
	fmt.Println("\n--- Error Logging ---")

	err := errors.New("database connection refused")

	// Standard error logging
	logger.LogError(err, "failed to connect to database",
		"host", "db.example.com",
		"port", 5432,
		"retry_count", 3,
	)

	// Error with stack trace (useful for unexpected errors)
	logger.ErrorWithStack("unexpected error with stack trace", err, true)
}

func demonstrateRequestLogging(logger *logging.Logger) {
	fmt.Println("\n--- Request Logging ---")

	// Create a sample HTTP request
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://api.example.com/v1/payments", nil)
	req.Header.Set("User-Agent", "PaymentClient/2.0")
	req.Header.Set("X-Request-ID", "req-abc123")
	req.Header.Set("Content-Type", "application/json")

	// Log request with response details
	logger.LogRequest(req,
		"status", http.StatusCreated,
		"duration_ms", 45,
		"response_size", 256,
	)
}

func demonstrateContextLogging(logger *logging.Logger) {
	fmt.Println("\n--- Context-Scoped Logging ---")

	// Simulate incoming request with IDs in context
	ctx := context.Background()
	ctx = context.WithValue(ctx, ctxKey("request_id"), "req-xyz789")
	ctx = context.WithValue(ctx, ctxKey("user_id"), "user-42")
	ctx = context.WithValue(ctx, ctxKey("trace_id"), "trace-abc")

	// Extract IDs and create request-scoped logger
	requestID, _ := ctx.Value(ctxKey("request_id")).(string)
	userID, _ := ctx.Value(ctxKey("user_id")).(string)
	traceID, _ := ctx.Value(ctxKey("trace_id")).(string)

	// Create a child logger with request context attached
	reqLogger := logger.Logger().With(
		"request_id", requestID,
		"user_id", userID,
		"trace_id", traceID,
	)

	// All subsequent logs automatically include context fields
	reqLogger.Info("handling payment request")
	reqLogger.Info("validating payment details", "amount", 99.99, "currency", "USD")
	reqLogger.Info("payment authorized", "auth_code", "AUTH-123")
	reqLogger.Info("request completed", "status", "success")
}

func demonstrateDebugInfo(logger *logging.Logger) {
	fmt.Println("\n--- Debug Info ---")

	// Emit some logs to generate metrics
	logger.Info("application ready")
	logger.Debug("debug message (filtered at INFO level)")
	logger.Warn("disk space low", "free_gb", 5)

	// Get diagnostic information about the logger
	debugInfo := logger.DebugInfo()
	fmt.Printf("Logger diagnostics: %s\n", debugInfo)
}

// Helper functions

type ctxKey string

func processOrder(orderID string) error {
	time.Sleep(15 * time.Millisecond) // Simulate work
	if orderID == "order-invalid" {
		return errors.New("order not found")
	}

	return nil
}
