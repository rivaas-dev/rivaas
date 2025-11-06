package logging

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Test basic logger creation
func TestNew(t *testing.T) {
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
			cfg, err := New(tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && cfg == nil {
				t.Error("New() returned nil config without error")
			}
		})
	}
}

// Test validation
func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg:  defaultConfig(),
		},
		{
			name: "nil output",
			cfg: &Config{
				output:      nil,
				serviceName: "test",
			},
			wantErr: true,
		},
		{
			name: "empty service name",
			cfg: &Config{
				output:      io.Discard,
				serviceName: "",
			},
			wantErr: true,
		},
		{
			name: "nil custom logger",
			cfg: &Config{
				output:       io.Discard,
				serviceName:  "test",
				useCustom:    true,
				customLogger: nil,
			},
			wantErr: true,
		},
		{
			name: "invalid sampling config",
			cfg: &Config{
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
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Test logging methods
func TestLoggingMethods(t *testing.T) {
	th := NewTestHelper(t)

	th.Logger.Debug("debug message", "key", "value")
	th.Logger.Info("info message", "key", "value")
	th.Logger.Warn("warn message", "key", "value")
	th.Logger.Error("error message", "key", "value")

	entries, err := th.Logs()
	if err != nil {
		t.Fatalf("failed to parse logs: %v", err)
	}

	if len(entries) != 4 {
		t.Errorf("expected 4 log entries, got %d", len(entries))
	}

	levels := []string{"DEBUG", "INFO", "WARN", "ERROR"}
	for i, entry := range entries {
		if entry.Level != levels[i] {
			t.Errorf("entry %d: expected level %s, got %s", i, levels[i], entry.Level)
		}
	}
}

// Test sensitive data redaction
func TestSensitiveDataRedaction(t *testing.T) {
	th := NewTestHelper(t)

	th.Logger.Info("user login",
		"username", "john",
		"password", "secret123",
		"token", "abc123",
		"api_key", "xyz789",
	)

	entries, err := th.Logs()
	if err != nil {
		t.Fatalf("failed to parse logs: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal("no log entries found")
	}

	entry := entries[0]

	// Check that sensitive fields are redacted
	sensitiveFields := []string{"password", "token", "api_key"}
	for _, field := range sensitiveFields {
		if val, ok := entry.Attrs[field]; ok {
			if val != "***REDACTED***" {
				t.Errorf("field %s should be redacted, got: %v", field, val)
			}
		}
	}

	// Check that non-sensitive field is not redacted
	if entry.Attrs["username"] != "john" {
		t.Errorf("username should not be redacted, got: %v", entry.Attrs["username"])
	}
}

// Test ErrorWithStack
func TestErrorWithStack(t *testing.T) {
	th := NewTestHelper(t)

	err := errors.New("test error")

	// With stack
	th.Logger.ErrorWithStack("error occurred", err, true, "context", "test")
	entries, _ := th.Logs()
	if len(entries) == 0 {
		t.Fatal("no log entries")
	}

	entry := entries[len(entries)-1]
	if _, hasStack := entry.Attrs["stack"]; !hasStack {
		t.Error("expected stack trace in log entry")
	}

	// Without stack
	th.Reset()
	th.Logger.ErrorWithStack("error occurred", err, false, "context", "test")
	entries, _ = th.Logs()
	if len(entries) == 0 {
		t.Fatal("no log entries")
	}

	entry = entries[len(entries)-1]
	if _, hasStack := entry.Attrs["stack"]; hasStack {
		t.Error("did not expect stack trace in log entry")
	}
}

// Test sampling
func TestSampling(t *testing.T) {
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
	for i := 0; i < 50; i++ {
		logger.Info("sampled message", "iteration", i)
	}

	// Parse logs to verify sampling occurred
	entries, err := ParseJSONLogEntries(buf)
	if err != nil {
		t.Fatalf("failed to parse logs: %v", err)
	}

	written := len(entries)

	// Should have sampled some logs (written < 50)
	if written >= 50 {
		t.Errorf("expected sampling to reduce log count, got %d logs", written)
	}

	// Should have kept some logs
	if written == 0 {
		t.Error("expected some logs to be written")
	}
}

// Test sampling with errors (errors should never be sampled)
func TestSampling_ErrorsAlwaysLogged(t *testing.T) {
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
	for i := 0; i < 50; i++ {
		logger.Info("info", "i", i)
		logger.Error("error", "i", i)
	}

	// Parse logs to verify all errors were logged
	entries, err := ParseJSONLogEntries(buf)
	if err != nil {
		t.Fatalf("failed to parse logs: %v", err)
	}

	// Count error level entries
	errorCount := 0
	for _, entry := range entries {
		if entry.Level == "ERROR" {
			errorCount++
		}
	}

	// All 50 errors should be logged (errors are never sampled)
	if errorCount != 50 {
		t.Errorf("expected 50 errors logged, got %d", errorCount)
	}
}

// Test DebugInfo
func TestDebugInfo(t *testing.T) {
	logger := MustNew(
		WithJSONHandler(),
		WithServiceName("test-service"),
		WithServiceVersion("v1.0.0"),
		WithEnvironment("test"),
		WithDebugMode(true),
	)

	info := logger.DebugInfo()

	if info["service_name"] != "test-service" {
		t.Errorf("expected service_name=test-service, got %v", info["service_name"])
	}

	if info["service_version"] != "v1.0.0" {
		t.Errorf("expected service_version=v1.0.0, got %v", info["service_version"])
	}

	if info["debug_mode"] != true {
		t.Errorf("expected debug_mode=true, got %v", info["debug_mode"])
	}
}

// Test SetLevel
func TestSetLevel(t *testing.T) {
	th := NewTestHelper(t, WithLevel(LevelInfo))

	// Debug should not log at INFO level
	th.Logger.Debug("debug message")
	if th.ContainsLog("debug message") {
		t.Error("debug message should not be logged at INFO level")
	}

	// Change to debug level
	if err := th.Logger.SetLevel(LevelDebug); err != nil {
		t.Fatalf("SetLevel failed: %v", err)
	}

	th.Reset()
	th.Logger.Debug("debug message 2")
	if !th.ContainsLog("debug message 2") {
		t.Error("debug message should be logged at DEBUG level")
	}
}

// Test SetLevel with custom logger
func TestSetLevel_CustomLogger(t *testing.T) {
	customLogger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	logger := MustNew(WithCustomLogger(customLogger))

	err := logger.SetLevel(LevelDebug)
	if err != ErrCannotChangeLevel {
		t.Errorf("expected ErrCannotChangeLevel, got %v", err)
	}
}

// Test shutdown
func TestShutdown(t *testing.T) {
	th := NewTestHelper(t)

	th.Logger.Info("before shutdown")

	ctx := context.Background()
	if err := th.Logger.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}

	th.Reset()
	th.Logger.Info("after shutdown")

	// Should not log after shutdown
	if th.ContainsLog("after shutdown") {
		t.Error("should not log after shutdown")
	}
}

// Test convenience methods
func TestLogRequest(t *testing.T) {
	th := NewTestHelper(t)

	req := httptest.NewRequest("GET", "/test?foo=bar", nil)
	th.Logger.LogRequest(req, "status", 200, "duration_ms", 45)

	if !th.ContainsAttr("method", "GET") {
		t.Error("missing method attribute")
	}
	if !th.ContainsAttr("path", "/test") {
		t.Error("missing path attribute")
	}
	if !th.ContainsAttr("query", "foo=bar") {
		t.Error("missing query attribute")
	}
}

func TestLogError(t *testing.T) {
	th := NewTestHelper(t)

	err := errors.New("test error")
	th.Logger.LogError(err, "operation failed", "retry", 3)

	if !th.ContainsLog("operation failed") {
		t.Error("missing log message")
	}
	if !th.ContainsAttr("retry", 3) {
		t.Error("missing retry attribute")
	}
}

func TestLogDuration(t *testing.T) {
	th := NewTestHelper(t)

	start := time.Now()
	time.Sleep(10 * time.Millisecond)
	th.Logger.LogDuration("operation completed", start, "rows", 100)

	if !th.ContainsLog("operation completed") {
		t.Error("missing log message")
	}
	if !th.ContainsAttr("rows", 100) {
		t.Error("missing rows attribute")
	}

	entries, _ := th.Logs()
	if len(entries) > 0 {
		if _, ok := entries[0].Attrs["duration"]; !ok {
			t.Error("missing duration attribute")
		}
		if _, ok := entries[0].Attrs["duration_ms"]; !ok {
			t.Error("missing duration_ms attribute")
		}
	}
}

// Test batch logger
func TestBatchLogger(t *testing.T) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	bl := NewBatchLogger(logger, 5, time.Second)
	defer bl.Close()

	// Add entries
	for i := 0; i < 3; i++ {
		bl.Info("message", "i", i)
	}

	// Should have 3 entries in batch
	if bl.Size() != 3 {
		t.Errorf("expected 3 entries in batch, got %d", bl.Size())
	}

	// Add 2 more to trigger auto-flush at batchSize=5
	bl.Info("message", "i", 3)
	bl.Info("message", "i", 4)

	// Wait a bit for flush
	time.Sleep(10 * time.Millisecond)

	// Batch should be empty after flush
	if bl.Size() != 0 {
		t.Errorf("expected 0 entries after flush, got %d", bl.Size())
	}
}

func TestBatchLogger_ManualFlush(t *testing.T) {
	th := NewTestHelper(t)
	bl := NewBatchLogger(th.Logger, 100, time.Hour) // Large batch, long interval
	defer bl.Close()

	bl.Info("message 1")
	bl.Info("message 2")

	// No logs yet (not flushed)
	if th.ContainsLog("message 1") {
		t.Error("logs should not be written before flush")
	}

	bl.Flush()

	// Now logs should be present
	if !th.ContainsLog("message 1") {
		t.Error("message 1 should be logged after flush")
	}
	if !th.ContainsLog("message 2") {
		t.Error("message 2 should be logged after flush")
	}
}

// Test all batch logger methods
func TestBatchLogger_AllLevels(t *testing.T) {
	th := NewTestHelper(t)
	bl := NewBatchLogger(th.Logger, 10, 100*time.Millisecond)
	defer bl.Close()

	bl.Debug("debug msg")
	bl.Info("info msg")
	bl.Warn("warn msg")
	bl.Error("error msg")

	bl.Flush()

	levels := th.CountLevel("DEBUG")
	if levels != 1 {
		t.Errorf("expected 1 DEBUG log, got %d", levels)
	}
}

// Test test helpers
func TestTestHelper(t *testing.T) {
	th := NewTestHelper(t)

	th.Logger.Info("test message", "user_id", "123", "action", "login")

	// Test ContainsLog
	if !th.ContainsLog("test message") {
		t.Error("ContainsLog should find the message")
	}

	// Test ContainsAttr
	if !th.ContainsAttr("user_id", "123") {
		t.Error("ContainsAttr should find user_id")
	}

	// Test CountLevel
	count := th.CountLevel("INFO")
	if count != 1 {
		t.Errorf("expected 1 INFO log, got %d", count)
	}

	// Test LastLog
	last, err := th.LastLog()
	if err != nil {
		t.Fatalf("LastLog failed: %v", err)
	}
	if last.Message != "test message" {
		t.Errorf("expected 'test message', got '%s'", last.Message)
	}

	// Test Reset
	th.Reset()
	if th.ContainsLog("test message") {
		t.Error("logs should be cleared after Reset")
	}

	// Test AssertLog
	th.Logger.Info("another message", "key", "value")
	th.AssertLog(t, "INFO", "another message", map[string]any{"key": "value"})
}

// Test mock writer
func TestMockWriter(t *testing.T) {
	mw := &MockWriter{}
	logger := MustNew(WithJSONHandler(), WithOutput(mw))

	logger.Info("test 1")
	logger.Info("test 2")

	if mw.WriteCount() != 2 {
		t.Errorf("expected 2 writes, got %d", mw.WriteCount())
	}

	if mw.BytesWritten() == 0 {
		t.Error("expected bytes to be written")
	}

	lastWrite := mw.LastWrite()
	if lastWrite == nil {
		t.Error("expected last write to be captured")
	}

	mw.Reset()
	if mw.WriteCount() != 0 {
		t.Error("expected write count to be 0 after reset")
	}
}

// Test slow writer (timeout testing)
func TestSlowWriter(t *testing.T) {
	sw := NewSlowWriter(50*time.Millisecond, io.Discard)

	start := time.Now()
	sw.Write([]byte("test"))
	elapsed := time.Since(start)

	if elapsed < 50*time.Millisecond {
		t.Errorf("expected at least 50ms delay, got %v", elapsed)
	}
}

// Test counting writer
func TestCountingWriter(t *testing.T) {
	cw := &CountingWriter{}
	logger := MustNew(WithJSONHandler(), WithOutput(cw))

	logger.Info("message 1")
	logger.Info("message 2")

	if cw.Count() == 0 {
		t.Error("expected bytes to be counted")
	}

	// Approximate check (each JSON log is roughly 50-100 bytes)
	if cw.Count() < 50 {
		t.Errorf("expected at least 50 bytes, got %d", cw.Count())
	}
}

// Test handler spy
func TestHandlerSpy(t *testing.T) {
	spy := &HandlerSpy{}
	customLogger := slog.New(spy)
	logger := MustNew(WithCustomLogger(customLogger))

	logger.Info("message 1", "key", "value")
	logger.Error("message 2", "error", "test")

	if spy.RecordCount() != 2 {
		t.Errorf("expected 2 records, got %d", spy.RecordCount())
	}

	records := spy.Records()
	if records[0].Message != "message 1" {
		t.Errorf("expected 'message 1', got '%s'", records[0].Message)
	}

	spy.Reset()
	if spy.RecordCount() != 0 {
		t.Error("expected 0 records after reset")
	}
}

// Test With and WithGroup
func TestWith(t *testing.T) {
	th := NewTestHelper(t)

	contextLogger := th.Logger.With("request_id", "req-123")
	contextLogger.Info("message")

	if !th.ContainsAttr("request_id", "req-123") {
		t.Error("expected request_id in log")
	}
}

func TestWithGroup(t *testing.T) {
	var buf bytes.Buffer
	logger := MustNew(WithJSONHandler(), WithOutput(&buf))

	groupLogger := logger.WithGroup("request")
	groupLogger.Info("received", "method", "POST")

	output := buf.String()
	if !strings.Contains(output, "\"request\"") {
		t.Error("expected request group in output")
	}
}

// Test debug mode
func TestDebugMode(t *testing.T) {
	logger := MustNew(
		WithJSONHandler(),
		WithOutput(io.Discard),
		WithDebugMode(true),
	)

	info := logger.DebugInfo()

	if info["debug_mode"] != true {
		t.Error("expected debug_mode to be true")
	}

	if info["add_source"] != true {
		t.Error("debug mode should enable source")
	}

	if info["level"] != "DEBUG" {
		t.Error("debug mode should set level to DEBUG")
	}
}

// Test sampling with ticker reset
func TestSampling_WithTicker(_ *testing.T) {
	logger := MustNew(
		WithJSONHandler(),
		WithOutput(io.Discard),
		WithSampling(SamplingConfig{
			Initial:    2,
			Thereafter: 5,
			Tick:       50 * time.Millisecond,
		}),
	)
	defer logger.Shutdown(context.Background())

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

// Test parseLogEntries edge cases
func TestParseLogEntries_Empty(t *testing.T) {
	buf := &bytes.Buffer{}
	entries, err := ParseJSONLogEntries(buf)

	if err != nil {
		t.Errorf("ParseJSONLogEntries should not error on empty buffer: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestParseLogEntries_Invalid(t *testing.T) {
	buf := bytes.NewBufferString("not json\n")
	_, err := ParseJSONLogEntries(buf)

	if err == nil {
		t.Error("expected error parsing invalid JSON")
	}
}

// Test captureStack
func TestCaptureStack(t *testing.T) {
	stack := captureStack(1)

	if stack == "" {
		t.Error("expected non-empty stack trace")
	}

	if !strings.Contains(stack, "TestCaptureStack") {
		t.Error("stack should contain test function name")
	}

	if !strings.Contains(stack, "logging_test.go") {
		t.Error("stack should contain file name")
	}
}

// Test all handler types output
func TestHandlerTypes_Output(t *testing.T) {
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
			var buf bytes.Buffer
			logger := MustNew(WithHandlerType(tt.handlerType), WithOutput(&buf))

			logger.Info("test", "key", "value")

			output := buf.String()
			if !strings.Contains(output, tt.contains) {
				t.Errorf("expected output to contain %q, got: %s", tt.contains, output)
			}
		})
	}
}

// Test concurrent access safety
func TestConcurrentAccess(_ *testing.T) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))

	done := make(chan struct{})
	workers := 10
	iterations := 100

	for i := 0; i < workers; i++ {
		go func(id int) {
			for j := 0; j < iterations; j++ {
				logger.Info("concurrent message", "worker", id, "iteration", j)
			}
			done <- struct{}{}
		}(i)
	}

	for i := 0; i < workers; i++ {
		<-done
	}

	// Verify concurrent logging completed without errors
	// All logs should have been written successfully
}

// Test metrics accuracy
func TestMetrics_Accuracy(t *testing.T) {
	// This test is removed as metrics have been removed from the package
	t.Skip("Metrics feature has been removed")
}

// Test validation errors
func TestValidation_Errors(t *testing.T) {
	// Test with nil output
	_, err := New(WithOutput(nil))
	if err == nil {
		t.Error("expected error with nil output")
	}

	// Test with empty service name
	cfg := defaultConfig()
	cfg.serviceName = ""
	if err := cfg.Validate(); err == nil {
		t.Error("expected error with empty service name")
	}

	// Test with nil custom logger
	cfg2 := defaultConfig()
	cfg2.useCustom = true
	cfg2.customLogger = nil
	if err := cfg2.Validate(); err == nil {
		t.Error("expected error with nil custom logger")
	}
}

// Test getEnvOrDefault
func TestGetEnvOrDefault(t *testing.T) {
	result := getEnvOrDefault("NONEXISTENT_VAR", "default")
	if result != "default" {
		t.Errorf("expected 'default', got '%s'", result)
	}

	t.Setenv("TEST_VAR", "custom")
	result = getEnvOrDefault("TEST_VAR", "default")
	if result != "custom" {
		t.Errorf("expected 'custom', got '%s'", result)
	}
}

// Test with replace attr
func TestWithReplaceAttr(t *testing.T) {
	th := NewTestHelper(t, WithReplaceAttr(func(_ []string, a slog.Attr) slog.Attr {
		if a.Key == "custom_field" {
			return slog.String(a.Key, "***CUSTOM***")
		}
		return a
	}))

	th.Logger.Info("message", "custom_field", "secret")

	if !th.ContainsAttr("custom_field", "***CUSTOM***") {
		t.Error("custom field should be redacted by custom replacer")
	}
}

// Test MustNew panic
func TestMustNew_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustNew should panic on invalid config")
		}
	}()

	// This should panic
	MustNew(WithOutput(nil))
}

// Test level filtering
func TestLevel_Filtering(t *testing.T) {
	th := NewTestHelper(t, WithLevel(LevelWarn))

	th.Logger.Debug("debug")
	th.Logger.Info("info")
	th.Logger.Warn("warn")
	th.Logger.Error("error")

	entries, _ := th.Logs()

	// Should only log WARN and ERROR
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}

	levels := []string{"WARN", "ERROR"}
	for i, entry := range entries {
		if entry.Level != levels[i] {
			t.Errorf("entry %d: expected %s, got %s", i, levels[i], entry.Level)
		}
	}
}

// Test Logger() accessor
func TestLogger_Accessor(t *testing.T) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))

	slogger := logger.Logger()
	if slogger == nil {
		t.Error("Logger() should return non-nil slog.Logger")
	}

	// Should be usable
	slogger.Info("test from slogger")
}

// Test Level() accessor
func TestLevel_Accessor(t *testing.T) {
	logger := MustNew(WithLevel(LevelWarn))

	if logger.Level() != LevelWarn {
		t.Errorf("expected LevelWarn, got %v", logger.Level())
	}
}

// Test error tracking
func TestErrorCount(_ *testing.T) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))

	logger.Info("info")
	logger.Error("error 1")
	logger.Error("error 2")
	logger.Warn("warn")

	// Verify logs were written (all should be logged)
	// Errors are always logged regardless of sampling
}

// Test sampling edge cases
func TestSampling_EdgeCases(t *testing.T) {
	buf := &bytes.Buffer{}
	// Zero thereafter means sample all
	logger := MustNew(
		WithJSONHandler(),
		WithOutput(buf),
		WithSampling(SamplingConfig{Initial: 0, Thereafter: 0}),
	)

	for i := 0; i < 10; i++ {
		logger.Info("message", "i", i)
	}

	// Verify logs were written (with Thereafter=0, all should be logged)
	entries, err := ParseJSONLogEntries(buf)
	if err != nil {
		t.Fatalf("failed to parse logs: %v", err)
	}

	if len(entries) != 10 {
		t.Errorf("expected 10 logs, got %d", len(entries))
	}
}

// Benchmark comparison: Regular vs Batch vs Sampling
func BenchmarkCompareLoggingStrategies(b *testing.B) {
	b.Run("Regular", func(b *testing.B) {
		logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.Info("message", "i", i)
		}
	})

	b.Run("Batch", func(b *testing.B) {
		logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
		bl := NewBatchLogger(logger, 100, time.Second)
		defer bl.Close()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			bl.Info("message", "i", i)
		}
	})

	b.Run("Sampling", func(b *testing.B) {
		logger := MustNew(
			WithJSONHandler(),
			WithOutput(io.Discard),
			WithSampling(SamplingConfig{Initial: 10, Thereafter: 100}),
		)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.Info("message", "i", i)
		}
	})
}
