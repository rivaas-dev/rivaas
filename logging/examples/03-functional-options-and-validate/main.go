// Package main demonstrates functional options and validation.
package main

import (
	"fmt"

	"rivaas.dev/logging"
)

func main() {
	// Demonstrate validation failure
	if _, err := logging.New(
		logging.WithOutput(nil), // invalid: output writer cannot be nil
	); err != nil {
		fmt.Println("validation failed:", err)
	}

	// Valid configuration using functional options
	logger := logging.MustNew(
		logging.WithConsoleHandler(),
		logging.WithLevel(logging.LevelInfo),
		logging.WithSource(true),
		logging.WithSampling(logging.SamplingConfig{Initial: 5, Thereafter: 50}),
	)

	logger.Info("logger initialized via validated config")
}
