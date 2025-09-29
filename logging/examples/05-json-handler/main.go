// Package main demonstrates JSON-formatted logging output.
package main

import (
	"rivaas.dev/logging"
)

func main() {
	// Create a logger with JSON output (structured, machine-readable)
	log := logging.MustNew(
		logging.WithJSONHandler(),
		logging.WithServiceName("my-api"),
		logging.WithServiceVersion("v1.0.0"),
		logging.WithEnvironment("production"),
		logging.WithSource(true), // Include source file and line number
	)

	// JSON structured logging
	log.Info("Service started",
		"port", 8080,
		"env", "production",
		"version", "v1.0.0",
	)

	// Complex structured data
	log.Info("Request processed",
		"request_id", "req-12345",
		"method", "POST",
		"path", "/api/users",
		"status_code", 201,
		"duration_ms", 45,
		"user_agent", "Mozilla/5.0",
	)

	// Error logging with context
	log.Error("Database query failed",
		"error", "connection timeout",
		"query", "SELECT * FROM users",
		"timeout_seconds", 30,
		"retry_count", 3,
		"database", "postgres",
	)
}
