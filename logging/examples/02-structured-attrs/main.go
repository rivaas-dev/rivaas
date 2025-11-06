// Package main demonstrates structured logging with attributes.
package main

import (
	"time"

	"rivaas.dev/logging"
)

func main() {
	logger := logging.MustNew(
		logging.WithConsoleHandler(),
		logging.WithLevel(logging.LevelInfo),
	)

	// Flat attributes
	logger.Info("user login",
		"user_id", 12345,
		"email", "alice@example.com",
		"mfa", true,
	)

	// Nested-like fields via consistent keys
	requestID := "req-abc123"
	logger.Info("request received",
		"request.id", requestID,
		"request.method", "POST",
		"request.path", "/api/payments",
	)

	// Redaction-friendly keys
	cardMasked := "**** **** **** 4242"
	logger.Info("payment processed",
		"payment.amount_cents", 2599,
		"payment.currency", "USD",
		"payment.card_masked", cardMasked,
		"latency_ms", 37,
		"timestamp", time.Now().Format(time.RFC3339),
	)
}
