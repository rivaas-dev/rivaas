// Copyright 2025 The Rivaas Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logging

import (
	"log/slog"
	"sync"
	"time"
)

// BatchLogger accumulates log records and flushes them in batches.
//
// Instead of writing each log entry immediately, entries are accumulated
// and written together.
//
// Trade-offs:
//   - Latency: Adds delay before logs appear (up to flush interval)
//   - Memory: Buffers entries until flush
//   - Durability: Crash before flush loses buffered entries
//
// Mitigation strategies:
//   - Periodic flush timer (logs appear within timeout even if batch not full)
//   - Crash recovery: External log aggregation systems provide durability
//   - Critical logs: Use synchronous logging for audit trails
//
// Typical configuration:
//   - Batch size: 100-1000 entries
//   - Flush interval: 1-5 seconds
//
// Thread-safe: Safe to use concurrently by multiple goroutines.
type BatchLogger struct {
	cfg       *Config
	entries   []batchEntry
	mu        sync.Mutex
	batchSize int
	ticker    *time.Ticker
	done      chan struct{}
}

type batchEntry struct {
	level slog.Level
	msg   string
	attrs []any
}

// NewBatchLogger creates a logger that batches entries before writing.
//
// Parameters:
//   - cfg: Underlying logger configuration for final output
//   - batchSize: Maximum entries before automatic flush (typical: 100-1000)
//   - flushInterval: Maximum time between flushes (typical: 1-5 seconds)
//
// Choosing batchSize:
//   - Too small (< 10): May not provide meaningful batching
//   - Too large (> 10000): Increases memory usage and delays
//   - Typical: 100-1000 for most applications
//
// Choosing flushInterval:
//   - Too short (< 100ms): May not provide meaningful batching
//   - Too long (> 30s): Unacceptable log delay
//   - Typical: 1-5 seconds for most applications
//
// Example:
//
//	logger := logging.MustNew(logging.WithJSONHandler())
//	batchLogger := logging.NewBatchLogger(logger, 100, time.Second)
//	defer batchLogger.Close()
//
//	// High-frequency logging
//	for i := 0; i < 10000; i++ {
//	    batchLogger.Info("high frequency event", "id", i)
//	}
//	// Logs are written in batches of 100 or every 1 second
func NewBatchLogger(cfg *Config, batchSize int, flushInterval time.Duration) *BatchLogger {
	bl := &BatchLogger{
		cfg:       cfg,
		entries:   make([]batchEntry, 0, batchSize),
		batchSize: batchSize,
		ticker:    time.NewTicker(flushInterval),
		done:      make(chan struct{}),
	}

	go bl.flusher()
	return bl
}

// Debug logs a debug message (batched).
func (bl *BatchLogger) Debug(msg string, args ...any) {
	bl.add(slog.LevelDebug, msg, args...)
}

// Info logs an info message (batched).
func (bl *BatchLogger) Info(msg string, args ...any) {
	bl.add(slog.LevelInfo, msg, args...)
}

// Warn logs a warning message (batched).
func (bl *BatchLogger) Warn(msg string, args ...any) {
	bl.add(slog.LevelWarn, msg, args...)
}

// Error logs an error message (batched).
func (bl *BatchLogger) Error(msg string, args ...any) {
	bl.add(slog.LevelError, msg, args...)
}

// add adds a log entry to the batch.
func (bl *BatchLogger) add(level slog.Level, msg string, args ...any) {
	bl.mu.Lock()
	defer bl.mu.Unlock()

	bl.entries = append(bl.entries, batchEntry{level, msg, args})
	if len(bl.entries) >= bl.batchSize {
		bl.flushLocked()
	}
}

// flusher runs in a goroutine and periodically flushes the batch.
func (bl *BatchLogger) flusher() {
	for {
		select {
		case <-bl.ticker.C:
			bl.Flush()
		case <-bl.done:
			return
		}
	}
}

// Flush writes all batched entries to the underlying logger.
func (bl *BatchLogger) Flush() {
	bl.mu.Lock()
	defer bl.mu.Unlock()
	bl.flushLocked()
}

// flushLocked flushes entries (must be called with lock held).
func (bl *BatchLogger) flushLocked() {
	if len(bl.entries) == 0 {
		return
	}

	for _, e := range bl.entries {
		switch e.level {
		case slog.LevelDebug:
			bl.cfg.Debug(e.msg, e.attrs...)
		case slog.LevelInfo:
			bl.cfg.Info(e.msg, e.attrs...)
		case slog.LevelWarn:
			bl.cfg.Warn(e.msg, e.attrs...)
		case slog.LevelError:
			bl.cfg.Error(e.msg, e.attrs...)
		}
	}
	bl.entries = bl.entries[:0]
}

// Close stops the batch logger and flushes any remaining entries.
//
// Important: Always call Close() to ensure buffered entries are written.
// Use defer immediately after creating the BatchLogger:
//
//	batchLogger := logging.NewBatchLogger(cfg, 100, time.Second)
//	defer batchLogger.Close()
//
// Failure to call Close() will result in lost log entries (up to batchSize).
func (bl *BatchLogger) Close() {
	close(bl.done)
	bl.ticker.Stop()
	bl.Flush()
}

// Size returns the current number of batched entries.
func (bl *BatchLogger) Size() int {
	bl.mu.Lock()
	defer bl.mu.Unlock()
	return len(bl.entries)
}
