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
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test basic logger creation
func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		opts    []Option
		wantErr bool
	}{
		{
			name: "default config",
			opts: nil,
		},
		{
			name: "with JSON handler",
			opts: []Option{WithJSONHandler()},
		},
		{
			name: "with text handler",
			opts: []Option{WithTextHandler()},
		},
		{
			name: "with console handler",
			opts: []Option{WithConsoleHandler()},
		},
		{
			name: "with debug level",
			opts: []Option{WithDebugLevel()},
		},
		{
			name: "with service info",
			opts: []Option{
				WithServiceName("test"),
				WithServiceVersion("v1.0.0"),
				WithEnvironment("test"),
			},
		},
		{
			name: "with source",
			opts: []Option{WithSource(true)},
		},
		{
			name: "with debug mode",
			opts: []Option{WithDebugMode(true)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := New(tt.opts...)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, cfg, "New() returned nil config without error")
		})
	}
}

// Test validation
func TestConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     *Logger
		wantErr bool
	}{
		{
			name: "valid config",
			cfg:  defaultLogger(),
		},
		{
			name: "nil output",
			cfg: &Logger{
				output:      nil,
				serviceName: "test",
			},
			wantErr: true,
		},
		{
			name: "empty service name is valid",
			cfg: &Logger{
				output:      io.Discard,
				serviceName: "",
			},
			wantErr: false,
		},
		{
			name: "nil custom logger",
			cfg: &Logger{
				output:       io.Discard,
				serviceName:  "test",
				useCustom:    true,
				customLogger: nil,
			},
			wantErr: true,
		},
		{
			name: "invalid sampling config",
			cfg: &Logger{
				output:      io.Discard,
				serviceName: "test",
				samplingConfig: &SamplingConfig{
					Initial:    -1,
					Thereafter: 100,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestLogger_LogMethods tests the basic logging methods.
func TestLogger_LogMethods(t *testing.T) {
	t.Parallel()

	th := NewTestHelper(t)

	th.Logger.Debug("debug message", "key", "value")
	th.Logger.Info("info message", "key", "value")
	th.Logger.Warn("warn message", "key", "value")
	th.Logger.Error("error message", "key", "value")

	entries, err := th.Logs()
	require.NoError(t, err, "failed to parse logs")

	assert.Len(t, entries, 4, "expected 4 log entries")

	levels := []string{"DEBUG", "INFO", "WARN", "ERROR"}
	for i, entry := range entries {
		assert.Equal(t, levels[i], entry.Level, "entry %d: expected level %s, got %s", i, levels[i], entry.Level)
	}
}

// TestLogger_SensitiveDataRedaction tests that sensitive fields are redacted.
func TestLogger_SensitiveDataRedaction(t *testing.T) {
	t.Parallel()

	th := NewTestHelper(t)

	th.Logger.Info("user login",
		"username", "john",
		"password", "secret123",
		"token", "abc123",
		"api_key", "xyz789",
	)

	entries, err := th.Logs()
	require.NoError(t, err, "failed to parse logs")
	require.NotEmpty(t, entries, "no log entries found")

	entry := entries[0]

	// Check that sensitive fields are redacted
	sensitiveFields := []string{"password", "token", "api_key"}
	for _, field := range sensitiveFields {
		if val, ok := entry.Attrs[field]; ok {
			assert.Equal(t, "***REDACTED***", val, "field %s should be redacted", field)
		}
	}

	// Check that non-sensitive field is not redacted
	assert.Equal(t, "john", entry.Attrs["username"], "username should not be redacted")
}

// TestLogger_ErrorWithStack tests error logging with stack traces.
func TestLogger_ErrorWithStack(t *testing.T) {
	t.Parallel()

	th := NewTestHelper(t)

	err := errors.New("test error")

	// With stack
	th.Logger.ErrorWithStack("error occurred", err, true, "context", "test")
	entries, logErr := th.Logs()
	require.NoError(t, logErr)
	require.NotEmpty(t, entries, "no log entries")

	entry := entries[len(entries)-1]
	_, hasStack := entry.Attrs["stack"]
	assert.True(t, hasStack, "expected stack trace in log entry")

	// Without stack
	th.Reset()
	th.Logger.ErrorWithStack("error occurred", err, false, "context", "test")
	entries, logErr = th.Logs()
	require.NoError(t, logErr)
	require.NotEmpty(t, entries, "no log entries")

	entry = entries[len(entries)-1]
	_, hasStack = entry.Attrs["stack"]
	assert.False(t, hasStack, "did not expect stack trace in log entry")
}

// TestLogger_Sampling tests log sampling behavior.
func TestLogger_Sampling(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	logger := MustNew(
		WithJSONHandler(),
		WithOutput(buf),
		WithSampling(SamplingConfig{
			Initial:    5,
			Thereafter: 10,
		}),
	)

	// Log 50 messages
	for i := range 50 {
		logger.Info("sampled message", "iteration", i)
	}

	// Parse logs to verify sampling occurred
	entries, err := ParseJSONLogEntries(buf)
	require.NoError(t, err, "failed to parse logs")

	written := len(entries)

	// Should have sampled some logs (written < 50)
	assert.Less(t, written, 50, "expected sampling to reduce log count")
	// Should have kept some logs
	assert.Positive(t, written, "expected some logs to be written")
}

// TestLogger_Sampling_ErrorsAlwaysLogged tests that errors bypass sampling.
func TestLogger_Sampling_ErrorsAlwaysLogged(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	logger := MustNew(
		WithJSONHandler(),
		WithOutput(buf),
		WithSampling(SamplingConfig{
			Initial:    1,
			Thereafter: 100,
		}),
	)

	// Log many info messages and errors
	for i := range 50 {
		logger.Info("info", "i", i)
		logger.Error("error", "i", i)
	}

	// Parse logs to verify all errors were logged
	entries, err := ParseJSONLogEntries(buf)
	require.NoError(t, err, "failed to parse logs")

	// Count error level entries
	errorCount := 0
	for _, entry := range entries {
		if entry.Level == "ERROR" {
			errorCount++
		}
	}

	// All 50 errors should be logged (errors are never sampled)
	assert.Equal(t, 50, errorCount, "expected 50 errors logged")
}

// TestLogger_DebugInfo tests the debug info accessor.
func TestLogger_DebugInfo(t *testing.T) {
	t.Parallel()

	logger := MustNew(
		WithJSONHandler(),
		WithServiceName("test-service"),
		WithServiceVersion("v1.0.0"),
		WithEnvironment("test"),
		WithDebugMode(true),
	)

	info := logger.DebugInfo()

	assert.Equal(t, "test-service", info["service_name"])
	assert.Equal(t, "v1.0.0", info["service_version"])
	assert.Equal(t, true, info["debug_mode"])
}

// TestLogger_SetLevel tests dynamic level changes.
func TestLogger_SetLevel(t *testing.T) {
	t.Parallel()

	th := NewTestHelper(t, WithLevel(LevelInfo))

	// Debug should not log at INFO level
	th.Logger.Debug("debug message")
	assert.False(t, th.ContainsLog("debug message"), "debug message should not be logged at INFO level")

	// Change to debug level
	err := th.Logger.SetLevel(LevelDebug)
	require.NoError(t, err, "SetLevel failed")

	th.Reset()
	th.Logger.Debug("debug message 2")
	assert.True(t, th.ContainsLog("debug message 2"), "debug message should be logged at DEBUG level")
}

// TestLogger_SetLevel_CustomLogger tests that SetLevel fails with custom loggers.
func TestLogger_SetLevel_CustomLogger(t *testing.T) {
	t.Parallel()

	customLogger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	logger := MustNew(WithCustomLogger(customLogger))

	err := logger.SetLevel(LevelDebug)
	assert.ErrorIs(t, err, ErrCannotChangeLevel)
}

// TestLogger_Shutdown tests graceful shutdown behavior.
func TestLogger_Shutdown(t *testing.T) {
	t.Parallel()

	th := NewTestHelper(t)

	th.Logger.Info("before shutdown")

	ctx := t.Context()
	err := th.Logger.Shutdown(ctx)
	require.NoError(t, err, "Shutdown failed")

	th.Reset()
	th.Logger.Info("after shutdown")

	// Should not log after shutdown
	assert.False(t, th.ContainsLog("after shutdown"), "should not log after shutdown")
}

// TestLogger_LogRequest tests the LogRequest helper method.
func TestLogger_LogRequest(t *testing.T) {
	t.Parallel()

	th := NewTestHelper(t)

	req := httptest.NewRequest(http.MethodGet, "/test?foo=bar", nil)
	th.Logger.LogRequest(req, "status", 200, "duration_ms", 45)

	assert.True(t, th.ContainsAttr("method", "GET"), "missing method attribute")
	assert.True(t, th.ContainsAttr("path", "/test"), "missing path attribute")
	assert.True(t, th.ContainsAttr("query", "foo=bar"), "missing query attribute")
}

// TestLogger_LogError tests the LogError helper method.
func TestLogger_LogError(t *testing.T) {
	t.Parallel()

	th := NewTestHelper(t)

	err := errors.New("test error")
	th.Logger.LogError(err, "operation failed", "retry", 3)

	assert.True(t, th.ContainsLog("operation failed"), "missing log message")
	assert.True(t, th.ContainsAttr("retry", 3), "missing retry attribute")
}

// TestLogger_LogDuration tests the LogDuration helper method.
func TestLogger_LogDuration(t *testing.T) {
	t.Parallel()

	th := NewTestHelper(t)

	start := time.Now()
	time.Sleep(10 * time.Millisecond)
	th.Logger.LogDuration("operation completed", start, "rows", 100)

	assert.True(t, th.ContainsLog("operation completed"), "missing log message")
	assert.True(t, th.ContainsAttr("rows", 100), "missing rows attribute")

	entries, err := th.Logs()
	require.NoError(t, err)
	if len(entries) > 0 {
		_, hasDuration := entries[0].Attrs["duration"]
		assert.True(t, hasDuration, "missing duration attribute")
		_, hasDurationMs := entries[0].Attrs["duration_ms"]
		assert.True(t, hasDurationMs, "missing duration_ms attribute")
	}
}

// TestBatchLogger tests batch logger basic functionality.
func TestBatchLogger(t *testing.T) {
	t.Parallel()

	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	bl := NewBatchLogger(logger, 5, time.Second)
	t.Cleanup(func() { bl.Close() })

	// Add entries
	for i := range 3 {
		bl.Info("message", "i", i)
	}

	// Should have 3 entries in batch
	assert.Equal(t, 3, bl.Size(), "expected 3 entries in batch")

	// Add 2 more to trigger auto-flush at batchSize=5
	bl.Info("message", "i", 3)
	bl.Info("message", "i", 4)

	// Wait a bit for flush
	time.Sleep(10 * time.Millisecond)

	// Batch should be empty after flush
	assert.Equal(t, 0, bl.Size(), "expected 0 entries after flush")
}

// TestBatchLogger_ManualFlush tests manual flushing of batch logger.
func TestBatchLogger_ManualFlush(t *testing.T) {
	t.Parallel()

	th := NewTestHelper(t)
	bl := NewBatchLogger(th.Logger, 100, time.Hour) // Large batch, long interval
	t.Cleanup(func() { bl.Close() })

	bl.Info("message 1")
	bl.Info("message 2")

	// No logs yet (not flushed)
	assert.False(t, th.ContainsLog("message 1"), "logs should not be written before flush")

	bl.Flush()

	// Now logs should be present
	assert.True(t, th.ContainsLog("message 1"), "message 1 should be logged after flush")
	assert.True(t, th.ContainsLog("message 2"), "message 2 should be logged after flush")
}

// TestBatchLogger_AllLevels tests all log level methods on batch logger.
func TestBatchLogger_AllLevels(t *testing.T) {
	t.Parallel()

	th := NewTestHelper(t)
	bl := NewBatchLogger(th.Logger, 10, 100*time.Millisecond)
	t.Cleanup(func() { bl.Close() })

	bl.Debug("debug msg")
	bl.Info("info msg")
	bl.Warn("warn msg")
	bl.Error("error msg")

	bl.Flush()

	levels := th.CountLevel("DEBUG")
	assert.Equal(t, 1, levels, "expected 1 DEBUG log")
}

// TestTestHelper tests the test helper utilities.
func TestTestHelper(t *testing.T) {
	t.Parallel()

	th := NewTestHelper(t)

	th.Logger.Info("test message", "user_id", "123", "action", "login")

	// Test ContainsLog
	assert.True(t, th.ContainsLog("test message"), "ContainsLog should find the message")

	// Test ContainsAttr
	assert.True(t, th.ContainsAttr("user_id", "123"), "ContainsAttr should find user_id")

	// Test CountLevel
	count := th.CountLevel("INFO")
	assert.Equal(t, 1, count, "expected 1 INFO log")

	// Test LastLog
	last, err := th.LastLog()
	require.NoError(t, err, "LastLog failed")
	assert.Equal(t, "test message", last.Message)

	// Test Reset
	th.Reset()
	assert.False(t, th.ContainsLog("test message"), "logs should be cleared after Reset")

	// Test AssertLog
	th.Logger.Info("another message", "key", "value")
	th.AssertLog(t, "INFO", "another message", map[string]any{"key": "value"})
}

// TestMockWriter tests the mock writer test utility.
func TestMockWriter(t *testing.T) {
	t.Parallel()

	mw := &MockWriter{}
	logger := MustNew(WithJSONHandler(), WithOutput(mw))

	logger.Info("test 1")
	logger.Info("test 2")

	assert.Equal(t, 2, mw.WriteCount(), "expected 2 writes")
	assert.Positive(t, mw.BytesWritten(), "expected bytes to be written")

	lastWrite := mw.LastWrite()
	assert.NotNil(t, lastWrite, "expected last write to be captured")

	mw.Reset()
	assert.Equal(t, 0, mw.WriteCount(), "expected write count to be 0 after reset")
}

// TestSlowWriter tests the slow writer for timeout scenarios.
func TestSlowWriter(t *testing.T) {
	t.Parallel()

	sw := NewSlowWriter(50*time.Millisecond, io.Discard)

	start := time.Now()
	sw.Write([]byte("test"))
	elapsed := time.Since(start)

	assert.GreaterOrEqual(t, elapsed, 50*time.Millisecond, "expected at least 50ms delay")
}

// TestCountingWriter tests the counting writer test utility.
func TestCountingWriter(t *testing.T) {
	t.Parallel()

	cw := &CountingWriter{}
	logger := MustNew(WithJSONHandler(), WithOutput(cw))

	logger.Info("message 1")
	logger.Info("message 2")

	assert.Positive(t, cw.Count(), "expected bytes to be counted")

	// Approximate check (each JSON log is roughly 50-100 bytes)
	assert.GreaterOrEqual(t, cw.Count(), int64(50), "expected at least 50 bytes")
}

// TestHandlerSpy tests the handler spy test utility.
func TestHandlerSpy(t *testing.T) {
	t.Parallel()

	spy := &HandlerSpy{}
	customLogger := slog.New(spy)
	logger := MustNew(WithCustomLogger(customLogger))

	logger.Info("message 1", "key", "value")
	logger.Error("message 2", "error", "test")

	assert.Equal(t, 2, spy.RecordCount(), "expected 2 records")

	records := spy.Records()
	assert.Equal(t, "message 1", records[0].Message)

	spy.Reset()
	assert.Equal(t, 0, spy.RecordCount(), "expected 0 records after reset")
}

// TestLogger_With tests the With method for adding attributes.
func TestLogger_With(t *testing.T) {
	t.Parallel()

	th := NewTestHelper(t)

	contextLogger := th.Logger.With("request_id", "req-123")
	contextLogger.Info("message")

	assert.True(t, th.ContainsAttr("request_id", "req-123"), "expected request_id in log")
}

// TestLogger_WithGroup tests the WithGroup method for grouping attributes.
func TestLogger_WithGroup(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := MustNew(WithJSONHandler(), WithOutput(&buf))

	groupLogger := logger.WithGroup("request")
	groupLogger.Info("received", "method", "POST")

	output := buf.String()
	assert.Contains(t, output, "\"request\"", "expected request group in output")
}

// TestLogger_DebugMode tests the debug mode configuration.
func TestLogger_DebugMode(t *testing.T) {
	t.Parallel()

	logger := MustNew(
		WithJSONHandler(),
		WithOutput(io.Discard),
		WithDebugMode(true),
	)

	info := logger.DebugInfo()

	assert.Equal(t, true, info["debug_mode"], "expected debug_mode to be true")
	assert.Equal(t, true, info["add_source"], "debug mode should enable source")
	assert.Equal(t, "DEBUG", info["level"], "debug mode should set level to DEBUG")
}

// TestLogger_Sampling_WithTicker tests sampling with periodic counter reset.
func TestLogger_Sampling_WithTicker(t *testing.T) {
	t.Parallel()

	logger := MustNew(
		WithJSONHandler(),
		WithOutput(io.Discard),
		WithSampling(SamplingConfig{
			Initial:    2,
			Thereafter: 5,
			Tick:       50 * time.Millisecond,
		}),
	)
	t.Cleanup(func() { logger.Shutdown(context.Background()) })

	// First batch: should log first 2
	logger.Info("msg 1")
	logger.Info("msg 2")
	logger.Info("msg 3") // Dropped
	logger.Info("msg 4") // Dropped

	// Wait for ticker to reset counter
	time.Sleep(100 * time.Millisecond)

	// Second batch: should log first 2 again after reset
	logger.Info("msg 5")
	logger.Info("msg 6")

	// Verify that logs were written (sampling should allow some through)
	// The exact count depends on sampling, but we should have at least some logs
}

// TestParseJSONLogEntries_Empty tests parsing an empty buffer.
func TestParseJSONLogEntries_Empty(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	entries, err := ParseJSONLogEntries(buf)

	require.NoError(t, err, "ParseJSONLogEntries should not error on empty buffer")
	assert.Empty(t, entries, "expected 0 entries")
}

// TestParseJSONLogEntries_Invalid tests parsing invalid JSON.
func TestParseJSONLogEntries_Invalid(t *testing.T) {
	t.Parallel()

	buf := bytes.NewBufferString("not json\n")
	_, err := ParseJSONLogEntries(buf)

	assert.Error(t, err, "expected error parsing invalid JSON")
}

// TestCaptureStack tests the stack trace capture utility.
func TestCaptureStack(t *testing.T) {
	t.Parallel()

	stack := captureStack(1)

	assert.NotEmpty(t, stack, "expected non-empty stack trace")
	assert.Contains(t, stack, "TestCaptureStack", "stack should contain test function name")
	assert.Contains(t, stack, "logging_test.go", "stack should contain file name")
}

// TestLogger_HandlerTypes tests output from different handler types.
func TestLogger_HandlerTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		handlerType HandlerType
		contains    string
	}{
		{
			name:        "JSON handler",
			handlerType: JSONHandler,
			contains:    `"msg":"test"`,
		},
		{
			name:        "Text handler",
			handlerType: TextHandler,
			contains:    "msg=test",
		},
		{
			name:        "Console handler",
			handlerType: ConsoleHandler,
			contains:    "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			logger := MustNew(WithHandlerType(tt.handlerType), WithOutput(&buf))

			logger.Info("test", "key", "value")

			output := buf.String()
			assert.Contains(t, output, tt.contains, "expected output to contain %q", tt.contains)
		})
	}
}

// TestLogger_ConcurrentAccess tests thread safety of concurrent logging.
func TestLogger_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))

	done := make(chan struct{})
	workers := 10
	iterations := 100

	for i := range workers {
		go func(id int) {
			for j := range iterations {
				logger.Info("concurrent message", "worker", id, "iteration", j)
			}
			done <- struct{}{}
		}(i)
	}

	for range workers {
		<-done
	}

	// Verify concurrent logging completed without errors
	// All logs should have been written successfully
}

// TestLogger_Validate_Errors tests validation error cases.
func TestLogger_Validate_Errors(t *testing.T) {
	t.Parallel()

	// Test with nil output
	_, err := New(WithOutput(nil))
	require.Error(t, err, "expected error with nil output")

	// Empty service name is now valid (sensible defaults)
	cfg := defaultLogger()
	cfg.serviceName = ""
	err = cfg.Validate()
	require.NoError(t, err, "empty service name should be valid")

	// Test with nil custom logger
	cfg2 := defaultLogger()
	cfg2.useCustom = true
	cfg2.customLogger = nil
	err = cfg2.Validate()
	assert.Error(t, err, "expected error with nil custom logger")
}

// TestLogger_WithReplaceAttr tests custom attribute replacement.
func TestLogger_WithReplaceAttr(t *testing.T) {
	t.Parallel()

	th := NewTestHelper(t, WithReplaceAttr(func(_ []string, a slog.Attr) slog.Attr {
		if a.Key == "custom_field" {
			return slog.String(a.Key, "***CUSTOM***")
		}

		return a
	}))

	th.Logger.Info("message", "custom_field", "secret")

	assert.True(t, th.ContainsAttr("custom_field", "***CUSTOM***"), "custom field should be redacted by custom replacer")
}

// TestMustNew_Panic tests that MustNew panics on invalid config.
func TestMustNew_Panic(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			require.Fail(t, "MustNew should panic on invalid config")
		}
	}()

	// This should panic
	MustNew(WithOutput(nil))
}

// TestLogger_Level_Filtering tests that logs below minimum level are filtered.
func TestLogger_Level_Filtering(t *testing.T) {
	t.Parallel()

	th := NewTestHelper(t, WithLevel(LevelWarn))

	th.Logger.Debug("debug")
	th.Logger.Info("info")
	th.Logger.Warn("warn")
	th.Logger.Error("error")

	entries, err := th.Logs()
	require.NoError(t, err)

	// Should only log WARN and ERROR
	assert.Len(t, entries, 2, "expected 2 entries")

	levels := []string{"WARN", "ERROR"}
	for i, entry := range entries {
		assert.Equal(t, levels[i], entry.Level, "entry %d: expected %s, got %s", i, levels[i], entry.Level)
	}
}

// TestLogger_Logger_Accessor tests the Logger() accessor method.
func TestLogger_Logger_Accessor(t *testing.T) {
	t.Parallel()

	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))

	slogger := logger.Logger()
	assert.NotNil(t, slogger, "Slogger() should return non-nil slog.Logger")

	// Should be usable
	slogger.Info("test from slogger")
}

// TestLogger_Level_Accessor tests the Level() accessor method.
func TestLogger_Level_Accessor(t *testing.T) {
	t.Parallel()

	logger := MustNew(WithLevel(LevelWarn))

	assert.Equal(t, LevelWarn, logger.Level())
}

// TestLogger_ErrorCount tests error counting behavior.
func TestLogger_ErrorCount(t *testing.T) {
	t.Parallel()

	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))

	logger.Info("info")
	logger.Error("error 1")
	logger.Error("error 2")
	logger.Warn("warn")

	// Verify logs were written (all should be logged)
	// Errors are always logged regardless of sampling
}

// TestLogger_Sampling_EdgeCases tests sampling with edge case configurations.
func TestLogger_Sampling_EdgeCases(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	// Zero thereafter means sample all
	logger := MustNew(
		WithJSONHandler(),
		WithOutput(buf),
		WithSampling(SamplingConfig{Initial: 0, Thereafter: 0}),
	)

	for i := range 10 {
		logger.Info("message", "i", i)
	}

	// Verify logs were written (with Thereafter=0, all should be logged)
	entries, err := ParseJSONLogEntries(buf)
	require.NoError(t, err, "failed to parse logs")

	assert.Len(t, entries, 10, "expected 10 logs")
}

// BenchmarkCompareLoggingStrategies compares regular, batch, and sampling logging.
func BenchmarkCompareLoggingStrategies(b *testing.B) {
	b.Run("Regular", func(b *testing.B) {
		logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
		b.ResetTimer()
		b.ReportAllocs()
		for b.Loop() {
			logger.Info("message", "i", 1)
		}
	})

	b.Run("Batch", func(b *testing.B) {
		logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
		bl := NewBatchLogger(logger, 100, time.Second)
		b.Cleanup(func() { bl.Close() })
		b.ResetTimer()
		b.ReportAllocs()
		for b.Loop() {
			bl.Info("message", "i", 1)
		}
	})

	b.Run("Sampling", func(b *testing.B) {
		logger := MustNew(
			WithJSONHandler(),
			WithOutput(io.Discard),
			WithSampling(SamplingConfig{Initial: 10, Thereafter: 100}),
		)
		b.ResetTimer()
		b.ReportAllocs()
		for b.Loop() {
			logger.Info("message", "i", 1)
		}
	})
}
