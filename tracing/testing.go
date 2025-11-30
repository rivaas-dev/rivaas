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

package tracing

import (
	"context"
	"net/http"
	"testing"
	"time"
)

// TestingTracer creates a test [Tracer] with sensible defaults for unit tests.
// The tracer uses [NoopProvider] to avoid any external dependencies.
// Use t.Cleanup to ensure proper shutdown.
//
// Example:
//
//	func TestSomething(t *testing.T) {
//	    t.Parallel()
//	    tracer := tracing.TestingTracer(t)
//	    // Use tracer...
//	}
func TestingTracer(t testing.TB, opts ...Option) *Tracer {
	t.Helper()

	// Default options for testing
	defaultOpts := []Option{
		WithServiceName("test-service"),
		WithServiceVersion("v1.0.0"),
		WithNoop(),
		WithSampleRate(1.0),
	}

	// Allow test-specific options to override defaults
	allOpts := append(defaultOpts, opts...)

	tracer, err := New(allOpts...)
	if err != nil {
		t.Fatalf("TestingTracer: failed to create tracer: %v", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tracer.Shutdown(ctx); err != nil {
			t.Logf("TestingTracer: shutdown warning: %v", err)
		}
	})

	return tracer
}

// TestingTracerWithStdout creates a test [Tracer] with [StdoutProvider].
// This is useful for debugging tests that need to see trace output.
// Use t.Cleanup to ensure proper shutdown.
//
// Example:
//
//	func TestSomething(t *testing.T) {
//	    t.Parallel()
//	    tracer := tracing.TestingTracerWithStdout(t)
//	    // Use tracer...
//	}
func TestingTracerWithStdout(t testing.TB, opts ...Option) *Tracer {
	t.Helper()

	// Default options for testing with stdout
	defaultOpts := []Option{
		WithServiceName("test-service"),
		WithServiceVersion("v1.0.0"),
		WithStdout(),
		WithSampleRate(1.0),
	}

	// Allow test-specific options to override defaults
	allOpts := append(defaultOpts, opts...)

	tracer, err := New(allOpts...)
	if err != nil {
		t.Fatalf("TestingTracerWithStdout: failed to create tracer: %v", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tracer.Shutdown(ctx); err != nil {
			t.Logf("TestingTracerWithStdout: shutdown warning: %v", err)
		}
	})

	return tracer
}

// TestingMiddleware creates test middleware with sensible defaults for unit tests.
// The middleware uses [NoopProvider] and includes common test configurations.
//
// Example:
//
//	func TestSomething(t *testing.T) {
//	    t.Parallel()
//	    middleware := tracing.TestingMiddleware(t)
//	    handler := middleware(myHandler)
//	    // Use handler...
//	}
func TestingMiddleware(t testing.TB, middlewareOpts ...MiddlewareOption) func(http.Handler) http.Handler {
	t.Helper()

	tracer := TestingTracer(t)
	middleware, err := Middleware(tracer, middlewareOpts...)
	if err != nil {
		t.Fatalf("TestingMiddleware: failed to create middleware: %v", err)
	}

	return middleware
}

// TestingMiddlewareWithTracer creates test middleware with a custom tracer.
// This is useful when you need to configure both the tracer and middleware options.
//
// Example:
//
//	func TestSomething(t *testing.T) {
//	    t.Parallel()
//	    tracer := tracing.TestingTracer(t, tracing.WithSampleRate(0.5))
//	    middleware := tracing.TestingMiddlewareWithTracer(t, tracer,
//	        tracing.WithExcludePaths("/health"),
//	    )
//	    handler := middleware(myHandler)
//	    // Use handler...
//	}
func TestingMiddlewareWithTracer(t testing.TB, tracer *Tracer, middlewareOpts ...MiddlewareOption) func(http.Handler) http.Handler {
	t.Helper()

	middleware, err := Middleware(tracer, middlewareOpts...)
	if err != nil {
		t.Fatalf("TestingMiddlewareWithTracer: failed to create middleware: %v", err)
	}

	return middleware
}
