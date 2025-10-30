package logging

import (
	"log/slog"
	"sync"
	"time"
)

// BatchLogger allows accumulating log entries and flushing them periodically.
// This is useful for high-frequency logging scenarios where you want to reduce
// the number of write operations.
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
// batchSize: Maximum number of entries before auto-flush
// flushInterval: Time interval for periodic flushing
//
// Example:
//
//	logger := logging.MustNew(logging.WithJSONHandler())
//	batchLogger := logging.NewBatchLogger(logger, 100, time.Second)
//	defer batchLogger.Close()
//
//	for i := 0; i < 10000; i++ {
//	    batchLogger.Info("high frequency event", "id", i)
//	}
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
