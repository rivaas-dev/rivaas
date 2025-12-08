// Package main demonstrates logging utilities for testing.
//
// This example covers:
//   - NewTestLogger for in-memory logging capture
//   - ParseJSONLogEntries for parsing and asserting on logs
//   - Common test patterns for verifying logging behavior
package main

import (
	"fmt"
	"strings"

	"rivaas.dev/logging"
)

func main() {
	// Basic test logger usage
	demonstrateBasicTestLogger()

	// Asserting on log content
	demonstrateLogAssertions()

	// Testing error logging
	demonstrateErrorLogTesting()
}

func demonstrateBasicTestLogger() {
	fmt.Println("--- Basic Test Logger ---")

	// Create a logger that writes to an in-memory buffer
	// This is useful for capturing logs during tests
	logger, buf := logging.NewTestLogger()

	// Emit some logs
	logger.Info("user created", "user_id", 123, "email", "test@example.com")
	logger.Warn("rate limit approaching", "current", 95, "limit", 100)
	logger.Error("validation failed", "field", "email", "error", "invalid format")

	// Parse the captured log entries
	entries, err := logging.ParseJSONLogEntries(buf)
	if err != nil {
		panic(err)
	}

	fmt.Printf("captured %d log entries\n", len(entries))
	for i, entry := range entries {
		fmt.Printf("  [%d] level=%s msg=%q\n", i, entry.Level, entry.Message)
	}
	fmt.Println()
}

func demonstrateLogAssertions() {
	fmt.Println("--- Log Assertions ---")

	logger, buf := logging.NewTestLogger()

	// Simulate code under test
	processUserSignup(logger, "alice@example.com")

	// Parse and assert on logs
	entries, _ := logging.ParseJSONLogEntries(buf)

	// Example assertion: verify specific log was emitted
	found := false
	for _, entry := range entries {
		if entry.Message == "user signup completed" {
			found = true
			// Verify attributes
			if email, ok := entry.Attrs["email"].(string); ok && email == "alice@example.com" {
				fmt.Println("✓ signup log contains correct email")
			}
			if _, ok := entry.Attrs["user_id"]; ok {
				fmt.Println("✓ signup log contains user_id")
			}
		}
	}
	if found {
		fmt.Println("✓ signup completion log found")
	}

	// Example: count logs by level
	errorCount := 0
	for _, entry := range entries {
		if entry.Level == "ERROR" {
			errorCount++
		}
	}
	fmt.Printf("✓ error count: %d (expected: 0)\n", errorCount)
	fmt.Println()
}

func demonstrateErrorLogTesting() {
	fmt.Println("--- Error Log Testing ---")

	logger, buf := logging.NewTestLogger()

	// Simulate code that should log errors
	processPayment(logger, "card-expired")

	entries, _ := logging.ParseJSONLogEntries(buf)

	// Find and verify error log
	for _, entry := range entries {
		if entry.Level == "ERROR" && strings.Contains(entry.Message, "payment") {
			fmt.Println("✓ payment error was logged")
			if errMsg, ok := entry.Attrs["error"].(string); ok {
				fmt.Printf("✓ error message: %q\n", errMsg)
			}
			if code, ok := entry.Attrs["error_code"].(string); ok {
				fmt.Printf("✓ error code: %s\n", code)
			}
		}
	}
	fmt.Println()
}

// Simulated functions under test

func processUserSignup(logger *logging.Logger, email string) {
	logger.Info("starting user signup", "email", email)

	// Simulate signup logic
	userID := 42

	logger.Info("user signup completed",
		"email", email,
		"user_id", userID,
		"plan", "free",
	)
}

func processPayment(logger *logging.Logger, cardStatus string) {
	logger.Info("processing payment", "card_status", cardStatus)

	if cardStatus == "card-expired" {
		logger.Error("payment failed",
			"error", "card has expired",
			"error_code", "CARD_EXPIRED",
			"card_status", cardStatus,
		)

		return
	}

	logger.Info("payment successful")
}
