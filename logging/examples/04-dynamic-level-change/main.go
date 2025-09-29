// Package main demonstrates dynamic log level changes at runtime.
package main

import (
	"os"
	"strings"

	"rivaas.dev/logging"
)

func main() {
	level := logging.LevelInfo
	if strings.EqualFold(os.Getenv("LOG_DEBUG"), "true") {
		level = logging.LevelDebug
	}

	logger := logging.MustNew(
		logging.WithConsoleHandler(),
		logging.WithLevel(level),
	)

	logger.Info("starting with level", "level", level.String())
	logger.Debug("this appears only when LOG_DEBUG=true")

	// Change at runtime
	_ = logger.SetLevel(logging.LevelDebug)
	logger.Debug("runtime debug enabled")
}
