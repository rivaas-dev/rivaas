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
	"context"
	"log/slog"
	"sync"
)

// bufferedRecord holds a captured log record for later replay.
type bufferedRecord struct {
	ctx    context.Context
	record slog.Record
}

// bufferState holds the shared mutable state for buffering handlers.
// All handlers derived from the same root share this state.
type bufferState struct {
	mu        sync.Mutex
	buffering bool
	records   []bufferedRecord
}

// bufferingHandler wraps a slog.Handler to buffer log records during startup.
// When buffering is enabled, records are stored instead of being written.
// When flushed, all buffered records are replayed to the underlying handler.
type bufferingHandler struct {
	underlying slog.Handler
	state      *bufferState // Shared state (pointer for proper synchronization)
}

// newBufferingHandler creates a new buffering handler wrapping the given handler.
func newBufferingHandler(h slog.Handler) *bufferingHandler {
	return &bufferingHandler{
		underlying: h,
		state: &bufferState{
			records: make([]bufferedRecord, 0, 32), // Pre-allocate for typical startup logs
		},
	}
}

// Enabled reports whether the handler handles records at the given level.
func (h *bufferingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.underlying.Enabled(ctx, level)
}

// Handle either buffers the record or passes it to the underlying handler.
func (h *bufferingHandler) Handle(ctx context.Context, r slog.Record) error {
	h.state.mu.Lock()
	if h.state.buffering {
		// Clone the record to avoid issues with record reuse
		h.state.records = append(h.state.records, bufferedRecord{
			ctx:    ctx,
			record: r.Clone(),
		})
		h.state.mu.Unlock()
		return nil
	}
	h.state.mu.Unlock()

	return h.underlying.Handle(ctx, r)
}

// WithAttrs returns a new handler with the given attributes.
// The new handler shares the same buffer and buffering state as the original.
func (h *bufferingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &bufferingHandler{
		underlying: h.underlying.WithAttrs(attrs),
		state:      h.state, // Share the same state (synchronized via pointer)
	}
}

// WithGroup returns a new handler with the given group name.
// The new handler shares the same buffer and buffering state as the original.
func (h *bufferingHandler) WithGroup(name string) slog.Handler {
	return &bufferingHandler{
		underlying: h.underlying.WithGroup(name),
		state:      h.state, // Share the same state (synchronized via pointer)
	}
}

// startBuffering enables buffering mode.
func (h *bufferingHandler) startBuffering() {
	h.state.mu.Lock()
	defer h.state.mu.Unlock()
	h.state.buffering = true
}

// flush replays all buffered records to the underlying handler and clears the buffer.
func (h *bufferingHandler) flush() error {
	h.state.mu.Lock()
	records := h.state.records
	h.state.records = make([]bufferedRecord, 0, 32)
	h.state.buffering = false
	h.state.mu.Unlock()

	// Replay all buffered records
	for _, br := range records {
		if err := h.underlying.Handle(br.ctx, br.record); err != nil {
			return err
		}
	}
	return nil
}

// isBuffering returns whether buffering is currently enabled.
func (h *bufferingHandler) isBuffering() bool {
	h.state.mu.Lock()
	defer h.state.mu.Unlock()
	return h.state.buffering
}

// StartBuffering enables log buffering on the logger.
// While buffering is enabled, log records are stored in memory instead of being written.
// Call FlushBuffer to replay all buffered logs to the output.
//
// This is useful for delaying startup logs until after a banner or other output is printed.
//
// Example:
//
//	logger.StartBuffering()
//	// ... initialization that produces logs ...
//	printBanner()
//	logger.FlushBuffer()
func (l *Logger) StartBuffering() {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Get current logger
	current := l.slogger.Load()
	if current == nil {
		return
	}

	// Check if already using a buffering handler
	if bh, ok := current.Handler().(*bufferingHandler); ok {
		bh.startBuffering()
		return
	}

	// Wrap the current handler with a buffering handler
	bh := newBufferingHandler(current.Handler())
	bh.startBuffering()

	newLogger := slog.New(bh)
	l.slogger.Store(newLogger)

	// Update global logger if registered
	if l.registerGlobal {
		slog.SetDefault(newLogger)
	}
}

// FlushBuffer replays all buffered log records to the output and disables buffering.
// If buffering was not enabled, this is a no-op.
//
// Example:
//
//	logger.StartBuffering()
//	// ... initialization that produces logs ...
//	printBanner()
//	logger.FlushBuffer() // Now all startup logs appear after the banner
func (l *Logger) FlushBuffer() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	current := l.slogger.Load()
	if current == nil {
		return nil
	}

	bh, ok := current.Handler().(*bufferingHandler)
	if !ok {
		return nil
	}

	return bh.flush()
}

// IsBuffering returns whether the logger is currently buffering logs.
func (l *Logger) IsBuffering() bool {
	current := l.slogger.Load()
	if current == nil {
		return false
	}

	bh, ok := current.Handler().(*bufferingHandler)
	if !ok {
		return false
	}

	return bh.isBuffering()
}
