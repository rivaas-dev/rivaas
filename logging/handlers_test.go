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
	"log/slog"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConsoleHandler_Enabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		opts     *slog.HandlerOptions
		level    slog.Level
		expected bool
	}{
		{
			name:     "default level INFO allows INFO",
			opts:     nil,
			level:    slog.LevelInfo,
			expected: true,
		},
		{
			name:     "default level INFO allows ERROR",
			opts:     nil,
			level:    slog.LevelError,
			expected: true,
		},
		{
			name:     "default level INFO rejects DEBUG",
			opts:     nil,
			level:    slog.LevelDebug,
			expected: false,
		},
		{
			name:     "custom level DEBUG allows DEBUG",
			opts:     &slog.HandlerOptions{Level: slog.LevelDebug},
			level:    slog.LevelDebug,
			expected: true,
		},
		{
			name:     "custom level WARN rejects INFO",
			opts:     &slog.HandlerOptions{Level: slog.LevelWarn},
			level:    slog.LevelInfo,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newConsoleHandler(&bytes.Buffer{}, tt.opts)
			got := h.Enabled(context.Background(), tt.level)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestConsoleHandler_Handle_WithAttrsAndSource(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	opts := &slog.HandlerOptions{Level: slog.LevelDebug, AddSource: true}
	h := newConsoleHandler(&buf, opts)

	r := slog.NewRecord(time.Now(), slog.LevelInfo, "test message", getTestPC())
	r.AddAttrs(slog.String("key", "value"), slog.Int("n", 42))

	err := h.Handle(context.Background(), r)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "test message")
	assert.Contains(t, output, "key=value")
	assert.Contains(t, output, "n=42")
	assert.Contains(t, output, ".go:", "expected file:line from AddSource")
}

func getTestPC() uintptr {
	var pcs [1]uintptr
	runtime.Callers(2, pcs[:])
	return pcs[0]
}

func TestConsoleHandler_WithAttrs(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	h := newConsoleHandler(&buf, nil)
	h, ok := h.WithAttrs([]slog.Attr{slog.String("pre", "attr")}).(*consoleHandler)
	require.True(t, ok)

	r := slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0)
	err := h.Handle(context.Background(), r)
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "pre=attr")
	assert.Contains(t, buf.String(), "msg")
}

func TestConsoleHandler_WithGroup(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	h := newConsoleHandler(&buf, nil)
	h, ok := h.WithGroup("g").WithAttrs([]slog.Attr{slog.String("k", "v")}).(*consoleHandler)
	require.True(t, ok)

	r := slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0)
	err := h.Handle(context.Background(), r)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "msg")
	assert.Contains(t, output, "k=v")
}

func TestConsoleHandler_levelColor(t *testing.T) {
	t.Parallel()

	h := newConsoleHandler(&bytes.Buffer{}, nil)

	assert.Contains(t, h.levelColor(slog.LevelError), "\033")
	assert.Contains(t, h.levelColor(slog.LevelWarn), "\033")
	assert.Contains(t, h.levelColor(slog.LevelInfo), "\033")
	assert.Contains(t, h.levelColor(slog.LevelDebug), "\033")
}

func TestConsoleHandler_appendAttr(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	opts := &slog.HandlerOptions{Level: slog.LevelDebug}
	h := newConsoleHandler(&buf, opts)

	tests := []struct {
		name           string
		attr           slog.Attr
		expectedSubstr string
	}{
		{"string", slog.String("s", "hello"), "s=hello"},
		{"int", slog.Int("i", 42), "i=42"},
		{"int64", slog.Int64("i64", 99), "i64=99"},
		{"bool", slog.Bool("b", true), "b=true"},
		{"duration", slog.Duration("d", time.Second), "d=1s"},
		{"time", slog.Time("t", time.Unix(0, 0).UTC()), "t=1970-01-01"},
		{"float64", slog.Float64("f64", 3.14), "f64=3.14"},
		{"float32", slog.Float64("f32", 2.5), "f32=2.5"},
		{"error", slog.Any("err", errors.New("test error")), "err=test error"},
		{"empty attr skipped", slog.Attr{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var b strings.Builder
			h.appendAttr(&b, tt.attr)
			if tt.expectedSubstr != "" {
				assert.Contains(t, b.String(), tt.expectedSubstr)
			}
		})
	}
}

func TestConsoleHandler_appendAttr_IntegerTypes(t *testing.T) {
	t.Parallel()

	h := newConsoleHandler(&bytes.Buffer{}, nil)

	var b strings.Builder
	h.appendAttr(&b, slog.Any("int8", int8(8)))
	assert.Contains(t, b.String(), "8")

	b.Reset()
	h.appendAttr(&b, slog.Any("int16", int16(16)))
	assert.Contains(t, b.String(), "16")

	b.Reset()
	h.appendAttr(&b, slog.Any("int32", int32(32)))
	assert.Contains(t, b.String(), "32")

	b.Reset()
	h.appendAttr(&b, slog.Any("uint", uint(1)))
	assert.Contains(t, b.String(), "1")

	b.Reset()
	h.appendAttr(&b, slog.Any("uint64", uint64(64)))
	assert.Contains(t, b.String(), "64")
}

func TestConsoleHandler_appendAttr_DefaultType(t *testing.T) {
	t.Parallel()

	h := newConsoleHandler(&bytes.Buffer{}, nil)
	var b strings.Builder
	// Custom type falls through to default (fmt.Fprint)
	h.appendAttr(&b, slog.Any("custom", struct{ X int }{X: 1}))
	assert.Contains(t, b.String(), "custom=")
}

func TestHandlerRecordSource(t *testing.T) {
	t.Parallel()

	pc := getTestPC()
	got := handlerRecordSource(pc)
	// Should be "filename:line" with no leading path
	assert.Contains(t, got, ".go:")
	assert.NotContains(t, got, "/")
	parts := strings.Split(got, ":")
	require.Len(t, parts, 2)
	assert.NotEmpty(t, parts[0])
	assert.NotEmpty(t, parts[1])
}
