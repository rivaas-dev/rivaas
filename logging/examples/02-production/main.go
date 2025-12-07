// Package main demonstrates production-ready logging configuration.
//
// This example covers:
//   - JSON handler for structured, machine-readable logs
//   - Service metadata (name, version, environment)
//   - Configuration validation
//   - Batch logging for high-throughput scenarios
//   - Sampling to reduce log volume
package main

import (
	"fmt"
	"time"

	"rivaas.dev/logging"
)

func main() {
	// Demonstrate configuration validation
	demonstrateValidation()

	// Create production-ready logger with JSON output
	logger := logging.MustNew(
		logging.WithJSONHandler(),              // Machine-readable JSON format
		logging.WithServiceName("payment-api"), // Service identifier
		logging.WithServiceVersion("v2.1.0"),   // For version tracking
		logging.WithEnvironment("production"),  // Environment tag
		logging.WithSource(true),               // Include source location
		logging.WithSampling(logging.SamplingConfig{
			Initial:    5,  // Log first 5 messages of each type
			Thereafter: 50, // Then log every 50th message
		}),
	)

	// Structured logging with service context
	logger.Info("service started",
		"port", 8080,
		"tls_enabled", true,
		"workers", 4,
	)

	// Request logging pattern
	logger.Info("request processed",
		"request_id", "req-12345",
		"method", "POST",
		"path", "/api/v2/payments",
		"status_code", 201,
		"duration_ms", 45,
		"user_agent", "PaymentSDK/3.0",
	)

	// Error logging with full context
	logger.Error("database query failed",
		"error", "connection timeout",
		"query", "SELECT * FROM transactions WHERE status = 'pending'",
		"timeout_seconds", 30,
		"retry_count", 3,
		"database", "postgres-primary",
	)

	// High-throughput batch logging
	demonstrateBatchLogging()
}

func demonstrateValidation() {
	// Invalid configuration: nil output writer
	_, err := logging.New(
		logging.WithOutput(nil),
	)
	if err != nil {
		fmt.Println("validation caught invalid config:", err)
	}

	// Invalid configuration can also be caught via Validate
	fmt.Println()
}

func demonstrateBatchLogging() {
	fmt.Println("\n--- Batch Logging Demo ---")

	// Create base logger
	base := logging.MustNew(
		logging.WithJSONHandler(),
		logging.WithServiceName("event-processor"),
	)

	// Wrap with batch logger for high-throughput scenarios
	// Buffers up to 128 entries, flushes every 500ms or when buffer is full
	batch := logging.NewBatchLogger(base, 128, 500*time.Millisecond)
	defer batch.Close() // Ensures final flush on shutdown

	// Simulate high-volume event processing
	for i := range 100 {
		batch.Info("event processed",
			"event_id", fmt.Sprintf("evt-%05d", i),
			"partition", i%4,
			"lag_ms", i*2,
		)
	}

	// Give time for async flush
	time.Sleep(600 * time.Millisecond)
	fmt.Println("batch logging complete")
}
