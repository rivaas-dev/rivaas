// Package main demonstrates basic logging setup for development.
//
// This example covers:
//   - Console handler initialization
//   - Log levels (Debug, Info, Warn, Error)
//   - Structured attributes with key-value pairs
//   - Dynamic level changes at runtime
package main

import (
	"errors"
	"os"
	"strings"
	"time"

	"rivaas.dev/logging"
)

func main() {
	// Initialize level from environment (common pattern)
	level := logging.LevelInfo
	if strings.EqualFold(os.Getenv("LOG_DEBUG"), "true") {
		level = logging.LevelDebug
	}

	// Create logger with console output (human-readable, for development)
	logger := logging.MustNew(
		logging.WithConsoleHandler(),
		logging.WithLevel(level),
		logging.WithSource(true), // Include file:line in output
	)

	// Basic logging at different levels
	logger.Info("service starting", "version", "v1.0.0", "port", 8080)
	logger.Debug("debug info (hidden at INFO level)")
	logger.Warn("using default configuration", "config_path", "/etc/app/config.yaml")

	// Structured attributes - flat key-value pairs
	logger.Info("user login",
		"user_id", 12345,
		"email", "alice@example.com",
		"mfa_enabled", true,
		"login_method", "oauth2",
	)

	// Nested-style keys for grouping related fields
	requestID := "req-abc123"
	logger.Info("request received",
		"request.id", requestID,
		"request.method", "POST",
		"request.path", "/api/payments",
		"request.content_type", "application/json",
	)

	// Simulated work with error handling
	if err := processPayment(); err != nil {
		logger.Error("payment processing failed",
			"error", err,
			"payment.amount_cents", 2599,
			"payment.currency", "USD",
			"retry_count", 0,
		)
	}

	// Dynamic level change at runtime (useful for debugging production issues)
	logger.Info("elevating log level to DEBUG")
	_ = logger.SetLevel(logging.LevelDebug)
	logger.Debug("debug logging now visible", "timestamp", time.Now().Unix())

	// Restore original level
	_ = logger.SetLevel(level)
	logger.Info("log level restored", "level", level.String())
}

func processPayment() error {
	time.Sleep(10 * time.Millisecond) // Simulate work
	return errors.New("insufficient funds")
}
