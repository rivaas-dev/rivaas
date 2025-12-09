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

package metrics

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"
)

// ErrServerNotReady is returned when the metrics server fails to start within the timeout.
var ErrServerNotReady = errors.New("metrics server not ready")

// TestingRecorder creates a test [Recorder] with sensible defaults for unit tests.
// The recorder uses [StdoutProvider] with server disabled to avoid port conflicts.
// Use t.Cleanup to ensure proper shutdown.
//
// Example:
//
//	func TestSomething(t *testing.T) {
//	    t.Parallel()
//	    recorder := metrics.TestingRecorder(t, "test-service")
//	    // Use recorder...
//	}
func TestingRecorder(tb testing.TB, serviceName string, opts ...Option) *Recorder {
	tb.Helper()

	// Default options for testing
	defaultOpts := []Option{
		WithServiceName(serviceName),
		WithStdout(),
		WithServerDisabled(),
	}

	// Allow test-specific options to override defaults
	allOpts := append(defaultOpts, opts...)

	recorder, err := New(allOpts...)
	if err != nil {
		tb.Fatalf("TestingRecorder: failed to create recorder: %v", err)
	}

	tb.Cleanup(func() {
		ctx, cancel := context.WithTimeout(tb.Context(), 5*time.Second)
		defer cancel()
		if shutdownErr := recorder.Shutdown(ctx); shutdownErr != nil {
			tb.Logf("TestingRecorder: shutdown warning: %v", shutdownErr)
		}
	})

	return recorder
}

// TestingRecorderWithPrometheus creates a test [Recorder] with [PrometheusProvider].
// The recorder uses a dynamic port to avoid conflicts.
// Use t.Cleanup to ensure proper shutdown.
//
// Example:
//
//	func TestSomething(t *testing.T) {
//	    t.Parallel()
//	    recorder := metrics.TestingRecorderWithPrometheus(t, "test-service")
//	    // Use recorder...
//	}
func TestingRecorderWithPrometheus(tb testing.TB, serviceName string, opts ...Option) *Recorder {
	tb.Helper()

	// Find an available port
	port := findAvailableTestPort(tb)

	// Default options for testing with Prometheus
	defaultOpts := []Option{
		WithServiceName(serviceName),
		WithPrometheus(fmt.Sprintf(":%d", port), "/metrics"),
	}

	// Allow test-specific options to override defaults
	allOpts := append(defaultOpts, opts...)

	recorder, err := New(allOpts...)
	if err != nil {
		tb.Fatalf("TestingRecorderWithPrometheus: failed to create recorder: %v", err)
	}

	// Start the metrics server
	if startErr := recorder.Start(tb.Context()); startErr != nil {
		tb.Fatalf("TestingRecorderWithPrometheus: failed to start server: %v", startErr)
	}

	tb.Cleanup(func() {
		ctx, cancel := context.WithTimeout(tb.Context(), 5*time.Second)
		defer cancel()
		if shutdownErr := recorder.Shutdown(ctx); shutdownErr != nil {
			tb.Logf("TestingRecorderWithPrometheus: shutdown warning: %v", shutdownErr)
		}
	})

	return recorder
}

// WaitForMetricsServer waits for the metrics server to be ready.
// This is useful for tests that need to verify the HTTP server is accepting connections.
func WaitForMetricsServer(tb testing.TB, address string, timeout time.Duration) error {
	tb.Helper()

	deadline := time.Now().Add(timeout)
	dialer := &net.Dialer{Timeout: 100 * time.Millisecond}
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(tb.Context(), 100*time.Millisecond)
		conn, err := dialer.DialContext(ctx, "tcp", address)
		cancel()
		if err == nil {
			conn.Close() //nolint:errcheck // Best-effort close, error not critical for test helper

			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}

	return fmt.Errorf("%w after %v", ErrServerNotReady, timeout)
}

// findAvailableTestPort finds an available TCP port for testing.
func findAvailableTestPort(tb testing.TB) int {
	tb.Helper()

	var lc net.ListenConfig
	listener, err := lc.Listen(tb.Context(), "tcp", ":0")
	if err != nil {
		tb.Fatalf("findAvailableTestPort: failed to find available port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close() //nolint:errcheck // Best-effort close, error not critical for port discovery

	return port
}
