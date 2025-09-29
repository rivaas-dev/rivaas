// Package main demonstrates basic logger initialization and log levels.
package main

import (
	"errors"
	"time"

	"rivaas.dev/logging"
)

func main() {
	logger := logging.MustNew(
		logging.WithConsoleHandler(),
		logging.WithLevel(logging.LevelInfo),
		logging.WithSource(true),
	)

	logger.Info("service starting", "version", "v1.0.0")
	logger.Debug("debug message won't appear at info level")
	logger.Warn("using default config")

	if err := doWork(); err != nil {
		logger.Error("work failed", "error", err)
	}

	// Elevate verbosity temporarily
	_ = logger.SetLevel(logging.LevelDebug)
	logger.Debug("now debug is visible", "ts", time.Now().Unix())

	// Source can be enabled via option at init; dynamic toggle is not supported
}

func doWork() error {
	time.Sleep(50 * time.Millisecond)
	return errors.New("example failure")
}
