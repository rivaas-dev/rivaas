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

package tracing

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fatalRecorder wraps testing.TB and records Fatalf calls instead of exiting.
// When Fatal was called, Cleanup is no-op so we do not run shutdown on a nil tracer.
type fatalRecorder struct {
	testing.TB
	fatalCalled bool
	fatalMsg    string
}

func (f *fatalRecorder) Fatalf(format string, args ...any) {
	f.fatalCalled = true
	f.fatalMsg = format
	// Do not call f.TB.Fatalf so execution continues and we can assert.
}

func (f *fatalRecorder) Cleanup(fn func()) {
	if f.fatalCalled {
		return // Skip cleanup so we never run tracer.Shutdown(nil).
	}
	f.TB.Cleanup(fn)
}

// TestTestingTracer_InvalidOptionsFails covers the error path when New() returns an error.
func TestTestingTracer_InvalidOptionsFails(t *testing.T) {
	t.Parallel()

	rec := &fatalRecorder{TB: t}
	// Conflicting providers cause New() to return a validation error.
	tracer := TestingTracer(rec, WithOTLP("localhost:4317"), WithStdout())

	assert.True(t, rec.fatalCalled, "TestingTracer should have called Fatalf when New fails")
	assert.Nil(t, tracer)
	assert.Contains(t, rec.fatalMsg, "TestingTracer")
}

// TestTestingTracerWithStdout_CreatesTracer covers TestingTracerWithStdout.
func TestTestingTracerWithStdout_CreatesTracer(t *testing.T) {
	t.Parallel()

	tracer := TestingTracerWithStdout(t)
	require.NotNil(t, tracer)
	assert.True(t, tracer.IsEnabled())

	ctx, span := tracer.StartSpan(t.Context(), "test-span")
	require.NotNil(t, ctx)
	require.NotNil(t, span)
	tracer.FinishSpan(span, http.StatusOK)
}

// TestTestingMiddleware_WrapsHandler covers TestingMiddleware.
func TestTestingMiddleware_WrapsHandler(t *testing.T) {
	t.Parallel()

	middleware := TestingMiddleware(t)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestTestingMiddlewareWithTracer_WrapsHandler covers TestingMiddlewareWithTracer.
func TestTestingMiddlewareWithTracer_WrapsHandler(t *testing.T) {
	t.Parallel()

	tracer := TestingTracer(t)
	middleware := TestingMiddlewareWithTracer(t, tracer, WithExcludePaths("/x"))
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
