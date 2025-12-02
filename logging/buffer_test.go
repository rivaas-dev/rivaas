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
	"strings"
	"testing"
)

func TestLogger_Buffering(t *testing.T) {
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
	if err := logger.FlushBuffer(); err != nil {
		t.Fatalf("FlushBuffer failed: %v", err)
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
	logger.FlushBuffer()

	firstOutput := buf.String()
	if !strings.Contains(firstOutput, "first round") {
		t.Error("expected 'first round' in output")
	}

	buf.Reset()

	// Second round of buffering
	logger.StartBuffering()
	logger.Info("second round")
	logger.FlushBuffer()

	secondOutput := buf.String()
	if !strings.Contains(secondOutput, "second round") {
		t.Error("expected 'second round' in output")
	}
}

func TestLogger_Buffering_NoBuffer(t *testing.T) {
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
	if err := logger.FlushBuffer(); err != nil {
		t.Errorf("FlushBuffer should not error when not buffering: %v", err)
	}
}

func TestLogger_IsBuffering_NotBuffering(t *testing.T) {
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
