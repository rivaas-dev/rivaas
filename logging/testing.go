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
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// LogEntry represents a parsed log entry for testing.
type LogEntry struct {
	Time    time.Time
	Level   string
	Message string
	Attrs   map[string]any
}

// NewTestLogger creates a [Logger] for testing with an in-memory buffer.
// The returned buffer can be used with [ParseJSONLogEntries] to inspect log output.
func NewTestLogger() (*Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	logger := MustNew(
		WithJSONHandler(),
		WithOutput(buf),
		WithLevel(LevelDebug),
	)

	return logger, buf
}

// ParseJSONLogEntries parses JSON log entries from buffer into [LogEntry] slices.
// It creates a copy of the buffer so the original is not consumed.
func ParseJSONLogEntries(buf *bytes.Buffer) ([]LogEntry, error) {
	// Create a copy to avoid consuming the original buffer
	data := buf.Bytes()
	reader := bytes.NewReader(data)

	var entries []LogEntry
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		var entry map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return nil, err
		}

		le := LogEntry{
			Message: entry["msg"].(string),
			Level:   entry["level"].(string),
			Attrs:   make(map[string]any),
		}

		for k, v := range entry {
			if k != "time" && k != "level" && k != "msg" {
				le.Attrs[k] = v
			}
		}

		entries = append(entries, le)
	}

	return entries, scanner.Err()
}

// TestHelper provides utilities for testing with the logging package.
type TestHelper struct {
	Logger *Logger
	Buffer *bytes.Buffer
}

// NewTestHelper creates a [TestHelper] with in-memory logging.
// Additional [Option] values can be passed to customize the logger.
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
		if v, vOk := entry.Attrs[key]; vOk {
			// Handle numeric type conversions (JSON unmarshals to float64)
			switch expected := value.(type) {
			case int:
				if actual, actualOk := v.(float64); actualOk {
					return int(actual) == expected
				}
			case int64:
				if actual, actualOk := v.(float64); actualOk {
					return int64(actual) == expected
				}
			case float64:
				if actual, actualOk := v.(float64); actualOk {
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
	require.NoError(t, err, "failed to parse logs")

	for _, entry := range entries {
		if entry.Level != level || entry.Message != msg {
			continue
		}

		// Check attributes
		match := true
		for k, expectedVal := range attrs {
			actualVal, actualValOk := entry.Attrs[k]
			if !actualValOk {
				match = false
				break
			}

			// Handle numeric type conversions (JSON unmarshals to float64)
			matched := false
			switch expected := expectedVal.(type) {
			case int:
				if actual, actualOk := actualVal.(float64); actualOk {
					matched = int(actual) == expected
				}
			case int64:
				if actual, actualOk := actualVal.(float64); actualOk {
					matched = int64(actual) == expected
				}
			case float64:
				if actual, actualOk := actualVal.(float64); actualOk {
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

	require.Fail(t, "log entry not found", "level=%s msg=%s attrs=%v", level, msg, attrs)
}

// ExampleTestHelper demonstrates using the test helper.
func ExampleTestHelper() {
	// This would typically be in a test function with *testing.T
	// th := NewTestHelper(t)
	//
	// th.Logger.Info("test message", "user_id", "123")
	//
	// th.AssertLog(t, "INFO", "test message", map[string]any{
	//     "user_id": "123",
	// })
}

// MockWriter is a mock io.Writer that records all writes for test assertions.
//
// Use cases:
//   - Verify number of write calls (batching behavior)
//   - Inspect write contents (log format validation)
//   - Simulate write errors (error handling tests)
//
// Example:
//
//	mw := &MockWriter{}
//	logger := logging.MustNew(logging.WithOutput(mw))
//	logger.Info("test")
//
//	if mw.WriteCount() != 1 {
//	    t.Error("expected exactly one write")
//	}
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
//
// Use cases:
//   - Tests that need to verify log volume without storing content
//   - Volume verification without memory constraints
//   - Long-running tests that would exhaust memory with MockWriter
//
// Example:
//
//	cw := &CountingWriter{}
//	logger := logging.MustNew(logging.WithOutput(cw))
//
//	for i := 0; i < 1000000; i++ {
//	    logger.Info("test")
//	}
//
//	t.Logf("Total bytes logged: %d", cw.Count())
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

// SlowWriter simulates slow I/O for testing timeouts and backpressure.
//
// Use cases:
//   - Test timeout handling in logging middleware
//   - Simulate network latency in distributed logging
//   - Verify non-blocking behavior under slow I/O
//   - Test context cancellation during logging
//
// Example:
//
//	// Simulate 100ms network latency
//	sw := NewSlowWriter(100*time.Millisecond, &bytes.Buffer{})
//	logger := logging.MustNew(logging.WithOutput(sw))
//
//	start := time.Now()
//	logger.Info("test")
//	duration := time.Since(start)
//
//	if duration < 100*time.Millisecond {
//	    t.Error("expected write to be delayed")
//	}
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

// HandlerSpy implements [slog.Handler] and records all Handle calls for testing.
//
// Use cases:
//   - Test custom handler implementations
//   - Verify handler receives expected records
//   - Test handler filtering logic
//   - Verify attribute and group handling
//
// Example:
//
//	spy := &HandlerSpy{}
//	logger := slog.New(spy)
//
//	logger.Info("test", "key", "value")
//
//	if spy.RecordCount() != 1 {
//	    t.Error("expected one record")
//	}
//
//	records := spy.Records()
//	if records[0].Message != "test" {
//	    t.Error("unexpected message")
//	}
type HandlerSpy struct {
	records []slog.Record
	mu      sync.Mutex
}

// Enabled implements [slog.Handler.Enabled].
func (hs *HandlerSpy) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

// Handle implements [slog.Handler.Handle].
func (hs *HandlerSpy) Handle(_ context.Context, r slog.Record) error {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	hs.records = append(hs.records, r)

	return nil
}

// WithAttrs implements [slog.Handler.WithAttrs].
func (hs *HandlerSpy) WithAttrs(_ []slog.Attr) slog.Handler {
	return hs
}

// WithGroup implements [slog.Handler.WithGroup].
func (hs *HandlerSpy) WithGroup(_ string) slog.Handler {
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
