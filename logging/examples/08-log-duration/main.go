// Package main demonstrates duration logging for monitoring request processing time.
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
	)

	// Success path
	start := time.Now()
	doWork(25 * time.Millisecond)
	logger.LogDuration("operation completed", start, "op", "success")

	// Error path (log error and duration separately)
	start = time.Now()
	if err := doFail(); err != nil {
		logger.LogDuration("operation failed", start, "op", "fail")
		logger.LogError(err, "failed to process", "op", "fail")
	}
}

func doWork(d time.Duration) { time.Sleep(d) }

func doFail() error { return errors.New("simulated error") }
