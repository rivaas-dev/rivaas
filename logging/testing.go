package logging

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"
)

// TestHelper provides utilities for testing with the logging package.
type TestHelper struct {
	Logger *Config
	Buffer *bytes.Buffer
}

// NewTestHelper creates a test helper with in-memory logging.
func NewTestHelper(t *testing.T, opts ...Option) *TestHelper {
	t.Helper()

	buf := &bytes.Buffer{}
	defaultOpts := []Option{
		WithJSONHandler(),
		WithOutput(buf),
		WithLevel(LevelDebug),
	}
	defaultOpts = append(defaultOpts, opts...)

	logger := MustNew(defaultOpts...)

	return &TestHelper{
		Logger: logger,
		Buffer: buf,
	}
}

// Logs returns all parsed log entries.
func (th *TestHelper) Logs() ([]LogEntry, error) {
	return ParseJSONLogEntries(th.Buffer)
}

// LastLog returns the most recent log entry.
func (th *TestHelper) LastLog() (*LogEntry, error) {
	entries, err := th.Logs()
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no log entries found")
	}
	return &entries[len(entries)-1], nil
}

// ContainsLog checks if any log entry contains the given message.
func (th *TestHelper) ContainsLog(msg string) bool {
	entries, err := th.Logs()
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if entry.Message == msg {
			return true
		}
	}
	return false
}

// ContainsAttr checks if any log entry contains the given attribute.
func (th *TestHelper) ContainsAttr(key string, value any) bool {
	entries, err := th.Logs()
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if v, ok := entry.Attrs[key]; ok {
			// Handle numeric type conversions (JSON unmarshals to float64)
			switch expected := value.(type) {
			case int:
				if actual, ok := v.(float64); ok {
					return int(actual) == expected
				}
			case int64:
				if actual, ok := v.(float64); ok {
					return int64(actual) == expected
				}
			case float64:
				if actual, ok := v.(float64); ok {
					return actual == expected
				}
			}

			// String comparison as fallback
			if fmt.Sprint(v) == fmt.Sprint(value) {
				return true
			}
		}
	}
	return false
}

// CountLevel returns the number of log entries at the given level.
func (th *TestHelper) CountLevel(level string) int {
	entries, err := th.Logs()
	if err != nil {
		return 0
	}

	count := 0
	for _, entry := range entries {
		if entry.Level == level {
			count++
		}
	}
	return count
}

// Reset clears the buffer for fresh testing.
func (th *TestHelper) Reset() {
	th.Buffer.Reset()
}

// AssertLog checks that a log entry exists with the given properties.
func (th *TestHelper) AssertLog(t *testing.T, level, msg string, attrs map[string]any) {
	t.Helper()

	entries, err := th.Logs()
	if err != nil {
		t.Fatalf("failed to parse logs: %v", err)
	}

	for _, entry := range entries {
		if entry.Level != level || entry.Message != msg {
			continue
		}

		// Check attributes
		match := true
		for k, expectedVal := range attrs {
			actualVal, ok := entry.Attrs[k]
			if !ok {
				match = false
				break
			}

			// Handle numeric type conversions (JSON unmarshals to float64)
			matched := false
			switch expected := expectedVal.(type) {
			case int:
				if actual, ok := actualVal.(float64); ok {
					matched = int(actual) == expected
				}
			case int64:
				if actual, ok := actualVal.(float64); ok {
					matched = int64(actual) == expected
				}
			case float64:
				if actual, ok := actualVal.(float64); ok {
					matched = actual == expected
				}
			case string:
				matched = fmt.Sprint(actualVal) == expected
			default:
				matched = fmt.Sprint(actualVal) == fmt.Sprint(expectedVal)
			}

			if !matched {
				match = false
				break
			}
		}

		if match {
			return // Found matching log
		}
	}

	t.Errorf("log entry not found: level=%s msg=%s attrs=%v", level, msg, attrs)
}

// Example_testHelper demonstrates using the test helper.
func Example_testHelper() {
	// This would typically be in a test function with *testing.T
	// th := NewTestHelper(t)
	//
	// th.Logger.Info("test message", "user_id", "123")
	//
	// th.AssertLog(t, "INFO", "test message", map[string]any{
	//     "user_id": "123",
	// })
}

// MockWriter is a mock io.Writer for testing write behavior.
type MockWriter struct {
	writes     [][]byte
	writeError error
	bytesTotal int
}

// Write implements io.Writer.
func (mw *MockWriter) Write(p []byte) (n int, err error) {
	if mw.writeError != nil {
		return 0, mw.writeError
	}

	mw.writes = append(mw.writes, append([]byte(nil), p...))
	mw.bytesTotal += len(p)
	return len(p), nil
}

// WriteCount returns the number of write calls.
func (mw *MockWriter) WriteCount() int {
	return len(mw.writes)
}

// BytesWritten returns total bytes written.
func (mw *MockWriter) BytesWritten() int {
	return mw.bytesTotal
}

// LastWrite returns the most recent write.
func (mw *MockWriter) LastWrite() []byte {
	if len(mw.writes) == 0 {
		return nil
	}
	return mw.writes[len(mw.writes)-1]
}

// Reset clears all recorded writes.
func (mw *MockWriter) Reset() {
	mw.writes = nil
	mw.bytesTotal = 0
}

// CountingWriter counts bytes written without storing them.
type CountingWriter struct {
	count int64
}

// Write implements io.Writer.
func (cw *CountingWriter) Write(p []byte) (n int, err error) {
	n = len(p)
	cw.count += int64(n)
	return n, nil
}

// Count returns the total bytes written.
func (cw *CountingWriter) Count() int64 {
	return cw.count
}

// SlowWriter is a writer that adds artificial delay for testing timeouts.
type SlowWriter struct {
	delay time.Duration
	inner io.Writer
}

// NewSlowWriter creates a writer that delays each write.
func NewSlowWriter(delay time.Duration, inner io.Writer) *SlowWriter {
	return &SlowWriter{delay: delay, inner: inner}
}

// Write implements io.Writer with delay.
func (sw *SlowWriter) Write(p []byte) (n int, err error) {
	time.Sleep(sw.delay)
	if sw.inner != nil {
		return sw.inner.Write(p)
	}
	return len(p), nil
}

// HandlerSpy records all Handle calls for testing custom handlers.
type HandlerSpy struct {
	records []slog.Record
	mu      sync.Mutex
}

// Enabled implements slog.Handler.
func (hs *HandlerSpy) Enabled(_ context.Context, level slog.Level) bool {
	return true
}

// Handle implements slog.Handler.
func (hs *HandlerSpy) Handle(_ context.Context, r slog.Record) error {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	hs.records = append(hs.records, r)
	return nil
}

// WithAttrs implements slog.Handler.
func (hs *HandlerSpy) WithAttrs(attrs []slog.Attr) slog.Handler {
	return hs
}

// WithGroup implements slog.Handler.
func (hs *HandlerSpy) WithGroup(name string) slog.Handler {
	return hs
}

// Records returns all captured records.
func (hs *HandlerSpy) Records() []slog.Record {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	return append([]slog.Record(nil), hs.records...)
}

// RecordCount returns the number of captured records.
func (hs *HandlerSpy) RecordCount() int {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	return len(hs.records)
}

// Reset clears all captured records.
func (hs *HandlerSpy) Reset() {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	hs.records = nil
}
