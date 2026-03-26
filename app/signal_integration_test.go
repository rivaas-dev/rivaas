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

//go:build integration && !windows

package app

import (
	"context"
	"net/http"
	"os"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSignal_SIGINTTrigersGracefulShutdown verifies that a SIGINT signal causes
// Start to initiate graceful shutdown and return nil.
func TestSignal_SIGINTTrigersGracefulShutdown(t *testing.T) {
	a, err := New(
		WithServiceName("test-signal-sigint"),
		WithServiceVersion("1.0.0"),
		WithPort(58010),
		WithServer(WithShutdownTimeout(2*time.Second)),
	)
	require.NoError(t, err)

	a.GET("/test", func(c *Context) {
		_ = c.String(http.StatusOK, "ok")
	})

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- a.Start(context.Background())
	}()

	// Wait for server to start and register signal handlers
	time.Sleep(300 * time.Millisecond)

	// Verify the server is serving
	resp, err := http.Get("http://localhost:58010/test")
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Send SIGINT to current process — framework should handle it
	err = syscall.Kill(os.Getpid(), syscall.SIGINT)
	require.NoError(t, err, "failed to send SIGINT")

	// Server should shut down cleanly
	select {
	case err := <-serverErr:
		assert.NoError(t, err, "Start should return nil on graceful shutdown")
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down in time after SIGINT")
	}
}

// TestSignal_SIGTERMTriggersGracefulShutdown verifies that SIGTERM also initiates
// graceful shutdown (Unix only).
func TestSignal_SIGTERMTriggersGracefulShutdown(t *testing.T) {
	a, err := New(
		WithServiceName("test-signal-sigterm"),
		WithServiceVersion("1.0.0"),
		WithPort(58011),
		WithServer(WithShutdownTimeout(2*time.Second)),
	)
	require.NoError(t, err)

	a.GET("/test", func(c *Context) {
		_ = c.String(http.StatusOK, "ok")
	})

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- a.Start(context.Background())
	}()

	time.Sleep(300 * time.Millisecond)

	err = syscall.Kill(os.Getpid(), syscall.SIGTERM)
	require.NoError(t, err, "failed to send SIGTERM")

	select {
	case err := <-serverErr:
		assert.NoError(t, err, "Start should return nil on graceful shutdown via SIGTERM")
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down in time after SIGTERM")
	}
}

// TestSignal_ForceShutdownOnSecondSignal verifies that a second signal during the
// shutdown window calls forceExit. The forceExit function is injected for testability
// so the test process is not killed.
func TestSignal_ForceShutdownOnSecondSignal(t *testing.T) {
	a, err := New(
		WithServiceName("test-signal-force"),
		WithServiceVersion("1.0.0"),
		WithPort(58012),
		// Long shutdown timeout so graceful shutdown does not complete before the second signal
		WithServer(WithShutdownTimeout(30*time.Second)),
	)
	require.NoError(t, err)

	// Register a slow OnShutdown hook to keep the server in the shutdown window
	// long enough to send the second signal.
	require.NoError(t, a.OnShutdown(func(ctx context.Context) {
		select {
		case <-ctx.Done():
		case <-time.After(10 * time.Second):
		}
	}))

	a.GET("/test", func(c *Context) {
		_ = c.String(http.StatusOK, "ok")
	})

	// Inject a mock forceExit that records the call instead of os.Exit.
	var forceExitCalled atomic.Bool
	var forceExitCode atomic.Int32
	a.forceExit = func(code int) {
		forceExitCode.Store(int32(code))
		forceExitCalled.Store(true)
	}

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- a.Start(context.Background())
	}()

	time.Sleep(300 * time.Millisecond)

	// First signal — triggers graceful shutdown (enters shutdown block)
	err = syscall.Kill(os.Getpid(), syscall.SIGINT)
	require.NoError(t, err, "failed to send first SIGINT")

	// Small delay to let the shutdown goroutine register the second-signal listener
	time.Sleep(100 * time.Millisecond)

	// Second signal — should invoke forceExit
	err = syscall.Kill(os.Getpid(), syscall.SIGINT)
	require.NoError(t, err, "failed to send second SIGINT")

	// Wait for forceExit to be called
	deadline := time.Now().Add(3 * time.Second)
	for !forceExitCalled.Load() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	assert.True(t, forceExitCalled.Load(), "forceExit should have been called on second signal")
	assert.Equal(t, int32(1), forceExitCode.Load(), "forceExit should be called with code 1")
}
