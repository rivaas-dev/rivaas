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

package logging_test

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/logging"
)

// Integration tests for the logging package.
// These tests verify behavior across multiple components and real-world scenarios.

func TestIntegration_MultiLoggerCoexistence(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	// Create multiple independent loggers with different configurations
	buf1 := &bytes.Buffer{}
	buf2 := &bytes.Buffer{}

	logger1 := logging.MustNew(
		logging.WithJSONHandler(),
		logging.WithOutput(buf1),
		logging.WithServiceName("service-a"),
		logging.WithLevel(logging.LevelInfo),
	)
	t.Cleanup(func() { logger1.Shutdown(context.Background()) })

	logger2 := logging.MustNew(
		logging.WithJSONHandler(),
		logging.WithOutput(buf2),
		logging.WithServiceName("service-b"),
		logging.WithLevel(logging.LevelDebug),
	)
	t.Cleanup(func() { logger2.Shutdown(context.Background()) })

	// Log from both loggers
	logger1.Info("message from service-a", "key", "value1")
	logger2.Debug("message from service-b", "key", "value2")

	// Verify each logger writes to its own buffer
	output1 := buf1.String()
	output2 := buf2.String()

	assert.Contains(t, output1, "service-a")
	assert.Contains(t, output1, "message from service-a")
	assert.NotContains(t, output1, "service-b")

	assert.Contains(t, output2, "service-b")
	assert.Contains(t, output2, "message from service-b")
	assert.NotContains(t, output2, "service-a")

	// Logger1 at INFO should not have DEBUG messages
	logger1.Debug("debug message")
	assert.NotContains(t, buf1.String(), "debug message")
}

func TestIntegration_ConcurrentLogging(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	buf := &bytes.Buffer{}
	logger := logging.MustNew(
		logging.WithJSONHandler(),
		logging.WithOutput(buf),
		logging.WithLevel(logging.LevelDebug),
	)
	t.Cleanup(func() { logger.Shutdown(context.Background()) })

	const goroutines = 50
	const messagesPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				logger.Info("concurrent message",
					"goroutine_id", id,
					"message_id", j,
				)
			}
		}(i)
	}

	wg.Wait()

	// Parse and verify logs
	entries, err := logging.ParseJSONLogEntries(buf)
	require.NoError(t, err)

	// Should have all messages
	expectedMessages := goroutines * messagesPerGoroutine
	assert.Equal(t, expectedMessages, len(entries), "expected %d log entries", expectedMessages)
}

func TestIntegration_BatchLoggerWithFlush(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	buf := &bytes.Buffer{}
	logger := logging.MustNew(
		logging.WithJSONHandler(),
		logging.WithOutput(buf),
	)
	t.Cleanup(func() { logger.Shutdown(context.Background()) })

	// Create batch logger with auto-flush
	bl := logging.NewBatchLogger(logger, 10, 100*time.Millisecond)

	// Add fewer messages than batch size
	for i := 0; i < 5; i++ {
		bl.Info("batch message", "i", i)
	}

	// Wait for auto-flush
	time.Sleep(200 * time.Millisecond)

	// Close batch logger before reading buffer to avoid race
	bl.Close()

	entries, err := logging.ParseJSONLogEntries(buf)
	require.NoError(t, err)
	assert.Len(t, entries, 5, "expected 5 messages after auto-flush")
}

func TestIntegration_SamplingUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	buf := &bytes.Buffer{}
	logger := logging.MustNew(
		logging.WithJSONHandler(),
		logging.WithOutput(buf),
		logging.WithSampling(logging.SamplingConfig{
			Initial:    10,
			Thereafter: 10, // Log 1 in every 10 after initial
		}),
	)
	t.Cleanup(func() { logger.Shutdown(context.Background()) })

	// Log 100 INFO messages
	for i := 0; i < 100; i++ {
		logger.Info("sampled message", "i", i)
	}

	entries, err := logging.ParseJSONLogEntries(buf)
	require.NoError(t, err)

	// Should have approximately: 10 (initial) + 9 (1 in every 10 for remaining 90) = 19 messages
	// Allow some tolerance due to atomic counter behavior
	assert.Greater(t, len(entries), 10, "expected more than initial 10 messages")
	assert.Less(t, len(entries), 100, "expected sampling to reduce messages")
}

func TestIntegration_ErrorsNeverSampled(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	buf := &bytes.Buffer{}
	logger := logging.MustNew(
		logging.WithJSONHandler(),
		logging.WithOutput(buf),
		logging.WithSampling(logging.SamplingConfig{
			Initial:    1,
			Thereafter: 1000, // Aggressive sampling
		}),
	)
	t.Cleanup(func() { logger.Shutdown(context.Background()) })

	// Log many errors - they should all appear
	const errorCount = 50
	for i := 0; i < errorCount; i++ {
		logger.Error("error message", "i", i)
	}

	entries, err := logging.ParseJSONLogEntries(buf)
	require.NoError(t, err)

	// Count ERROR level entries
	errorEntries := 0
	for _, entry := range entries {
		if entry.Level == "ERROR" {
			errorEntries++
		}
	}

	assert.Equal(t, errorCount, errorEntries, "all errors should be logged regardless of sampling")
}

func TestIntegration_ContextLoggerPropagation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	buf := &bytes.Buffer{}
	logger := logging.MustNew(
		logging.WithJSONHandler(),
		logging.WithOutput(buf),
	)
	t.Cleanup(func() { logger.Shutdown(context.Background()) })

	// Simulate request context with tracing
	ctx := context.Background()
	cl := logging.NewContextLogger(ctx, logger)

	// Add request-scoped fields using With()
	clWithFields := cl.With("request_id", "req-123", "user_id", "user-456")

	// Log in different "layers" of the application
	clWithFields.Info("received request")
	clWithFields.Info("processing data", "step", "validation")
	clWithFields.Info("request completed", "status", "success")

	entries, err := logging.ParseJSONLogEntries(buf)
	require.NoError(t, err)

	require.Len(t, entries, 3)

	// All entries should have the context fields
	for _, entry := range entries {
		assert.Equal(t, "req-123", entry.Attrs["request_id"], "missing request_id")
		assert.Equal(t, "user-456", entry.Attrs["user_id"], "missing user_id")
	}
}

func TestIntegration_DynamicLevelChange(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	buf := &bytes.Buffer{}
	logger := logging.MustNew(
		logging.WithJSONHandler(),
		logging.WithOutput(buf),
		logging.WithLevel(logging.LevelInfo),
	)
	t.Cleanup(func() { logger.Shutdown(context.Background()) })

	// Debug should not log at INFO level
	logger.Debug("debug before change")

	// Change level to DEBUG
	err := logger.SetLevel(logging.LevelDebug)
	require.NoError(t, err)

	// Now debug should log
	logger.Debug("debug after change")

	entries, err := logging.ParseJSONLogEntries(buf)
	require.NoError(t, err)

	// Should only have the second debug message
	require.Len(t, entries, 1)
	assert.Equal(t, "debug after change", entries[0].Message)
}

func TestIntegration_GracefulShutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	buf := &bytes.Buffer{}
	logger := logging.MustNew(
		logging.WithJSONHandler(),
		logging.WithOutput(buf),
	)

	// Log some messages
	logger.Info("before shutdown")

	// Shutdown
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := logger.Shutdown(ctx)
	require.NoError(t, err)

	// Messages after shutdown should be ignored
	logger.Info("after shutdown")

	entries, err := logging.ParseJSONLogEntries(buf)
	require.NoError(t, err)

	// Should only have the message before shutdown
	require.Len(t, entries, 1)
	assert.Equal(t, "before shutdown", entries[0].Message)
}

func TestIntegration_SensitiveDataRedaction(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	buf := &bytes.Buffer{}
	logger := logging.MustNew(
		logging.WithJSONHandler(),
		logging.WithOutput(buf),
	)
	t.Cleanup(func() { logger.Shutdown(context.Background()) })

	// Log with sensitive fields
	logger.Info("user authentication",
		"username", "john_doe",
		"password", "super_secret_123",
		"token", "jwt_token_xyz",
		"api_key", "api_key_abc",
	)

	entries, err := logging.ParseJSONLogEntries(buf)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	entry := entries[0]

	// Non-sensitive field should be present
	assert.Equal(t, "john_doe", entry.Attrs["username"])

	// Sensitive fields should be redacted
	assert.Equal(t, "***REDACTED***", entry.Attrs["password"])
	assert.Equal(t, "***REDACTED***", entry.Attrs["token"])
	assert.Equal(t, "***REDACTED***", entry.Attrs["api_key"])
}
