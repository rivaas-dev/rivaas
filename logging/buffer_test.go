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

//go:build !integration

package logging

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogger_Buffering(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer

	logger, err := New(
		WithOutput(&buf),
		WithTextHandler(),
		WithLevel(LevelInfo),
	)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	// Start buffering
	logger.StartBuffering()

	if !logger.IsBuffering() {
		t.Error("expected IsBuffering() to return true after StartBuffering()")
	}

	// Log some messages while buffering
	logger.Info("buffered message 1")
	logger.Info("buffered message 2")

	// Nothing should be written yet
	if buf.Len() > 0 {
		t.Errorf("expected no output while buffering, got: %s", buf.String())
	}

	// Flush the buffer
	if flushErr := logger.FlushBuffer(); flushErr != nil {
		t.Fatalf("FlushBuffer failed: %v", flushErr)
	}

	if logger.IsBuffering() {
		t.Error("expected IsBuffering() to return false after FlushBuffer()")
	}

	// Now the messages should be written
	output := buf.String()
	if !strings.Contains(output, "buffered message 1") {
		t.Error("expected 'buffered message 1' in output after flush")
	}
	if !strings.Contains(output, "buffered message 2") {
		t.Error("expected 'buffered message 2' in output after flush")
	}
}

func TestLogger_Buffering_Multiple(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer

	logger, err := New(
		WithOutput(&buf),
		WithTextHandler(),
		WithLevel(LevelInfo),
	)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	// First round of buffering
	logger.StartBuffering()
	logger.Info("first round")
	err = logger.FlushBuffer()
	if err != nil {
		t.Fatalf("failed to flush buffer: %v", err)
	}

	firstOutput := buf.String()
	if !strings.Contains(firstOutput, "first round") {
		t.Error("expected 'first round' in output")
	}

	buf.Reset()

	// Second round of buffering
	logger.StartBuffering()
	logger.Info("second round")
	err = logger.FlushBuffer()
	if err != nil {
		t.Fatalf("failed to flush buffer: %v", err)
	}

	secondOutput := buf.String()
	if !strings.Contains(secondOutput, "second round") {
		t.Error("expected 'second round' in output")
	}
}

func TestLogger_Buffering_NoBuffer(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer

	logger, err := New(
		WithOutput(&buf),
		WithTextHandler(),
		WithLevel(LevelInfo),
	)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	// Without buffering, messages should go directly to output
	logger.Info("direct message")

	output := buf.String()
	if !strings.Contains(output, "direct message") {
		t.Error("expected 'direct message' in output without buffering")
	}

	// FlushBuffer should be a no-op when not buffering
	if flushErr := logger.FlushBuffer(); flushErr != nil {
		t.Errorf("FlushBuffer should not error when not buffering: %v", flushErr)
	}
}

func TestLogger_IsBuffering_NotBuffering(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer

	logger, err := New(
		WithOutput(&buf),
		WithTextHandler(),
		WithLevel(LevelInfo),
	)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	if logger.IsBuffering() {
		t.Error("expected IsBuffering() to return false initially")
	}
}

// TestLogger_Buffering_WithAttrs tests that WithAttrs on the buffering handler is exercised (coverage).
func TestLogger_Buffering_WithAttrs(t *testing.T) {
	t.Parallel()

	th := NewTestHelper(t)
	th.Logger.StartBuffering()
	th.Logger.Logger().With("attr_key", "attr_value").Info("message with attrs")
	err := th.Logger.FlushBuffer()
	require.NoError(t, err)

	assert.True(t, th.ContainsLog("message with attrs"))
	entries, err := th.Logs()
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

// TestLogger_Buffering_WithGroup tests that WithGroup on the buffering handler is exercised (coverage).
func TestLogger_Buffering_WithGroup(t *testing.T) {
	t.Parallel()

	th := NewTestHelper(t)
	th.Logger.StartBuffering()
	th.Logger.Logger().WithGroup("mygroup").With("k", "v").Info("message in group")
	err := th.Logger.FlushBuffer()
	require.NoError(t, err)

	assert.True(t, th.ContainsLog("message in group"))
	entries, err := th.Logs()
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

// failingHandler is a slog.Handler that returns an error from Handle (for testing flush error path).
type failingHandler struct {
	slog.Handler
}

func (h *failingHandler) Handle(ctx context.Context, r slog.Record) error {
	return errors.New("handle error")
}

// TestLogger_FlushBuffer_WhenHandlerReturnsError tests that FlushBuffer returns error when underlying Handle fails.
func TestLogger_FlushBuffer_WhenHandlerReturnsError(t *testing.T) {
	t.Parallel()

	fh := &failingHandler{Handler: slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug})}
	logger := MustNew(WithCustomLogger(slog.New(fh)))
	logger.StartBuffering()
	logger.Info("message")

	err := logger.FlushBuffer()
	require.Error(t, err)
	assert.ErrorContains(t, err, "handle error")
}
