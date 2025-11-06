// Package main demonstrates debug information and metrics logging.
package main

import (
	"fmt"

	"rivaas.dev/logging"
)

func main() {
	logger := logging.MustNew(
		logging.WithConsoleHandler(),
		logging.WithLevel(logging.LevelInfo),
		logging.WithSource(true),
	)

	// Emit some logs
	logger.Info("warmup complete")
	logger.Debug("debug hidden at info level")
	logger.Warn("low disk space", "free_gb", 3)

	// Print diagnostic info
	fmt.Println("debug info:", logger.DebugInfo())
}
