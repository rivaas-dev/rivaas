// Package main demonstrates batch logging for high-throughput scenarios.
package main

import (
	"time"

	"rivaas.dev/logging"
)

func main() {
	base := logging.MustNew(logging.WithConsoleHandler())
	batch := logging.NewBatchLogger(base, 128, 500*time.Millisecond)
	defer batch.Close()

	for i := 0; i < 1000; i++ {
		batch.Info("event", "index", i)
	}
}
