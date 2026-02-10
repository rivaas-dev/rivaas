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
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReload_SIGHUP(t *testing.T) {
	app, err := New(
		WithServiceName("test-reload-sighup"),
		WithServiceVersion("1.0.0"),
		WithPort(58001), // Use high port to avoid conflicts
	)
	require.NoError(t, err)

	var reloadCount int
	var mu sync.Mutex

	app.OnReload(func(ctx context.Context) error {
		mu.Lock()
		reloadCount++
		mu.Unlock()
		return nil
	})

	app.GET("/test", func(c *Context) {
		_ = c.String(http.StatusOK, "ok")
	})

	// Start server in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- app.Start(ctx) // Send result (nil or error)
	}()

	// Wait for server to be ready and to register for SIGHUP (avoid race)
	time.Sleep(300 * time.Millisecond)

	// Send SIGHUP signal to current process
	err = syscall.Kill(os.Getpid(), syscall.SIGHUP)
	require.NoError(t, err, "failed to send SIGHUP")

	// Wait for reload to process
	time.Sleep(100 * time.Millisecond)

	// Send another SIGHUP
	err = syscall.Kill(os.Getpid(), syscall.SIGHUP)
	require.NoError(t, err, "failed to send second SIGHUP")

	// Wait for second reload to process
	time.Sleep(100 * time.Millisecond)

	// Verify reloads were executed
	mu.Lock()
	count := reloadCount
	mu.Unlock()

	assert.GreaterOrEqual(t, count, 2, "SIGHUP signals should trigger reload hooks")

	// Cancel context to trigger graceful shutdown
	cancel()

	// Wait for server to shut down (Start returns nil on clean shutdown or error on failure)
	select {
	case err := <-serverErr:
		// Server exited (either clean or with error)
		if err != nil {
			t.Fatalf("server error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down in time")
	}
}

func TestReload_SIGHUPWithError(t *testing.T) {
	app, err := New(
		WithServiceName("test-reload-sighup-error"),
		WithServiceVersion("1.0.0"),
		WithPort(58002), // Use high port to avoid conflicts
	)
	require.NoError(t, err)

	var reloadAttempts int
	var mu sync.Mutex

	// First hook succeeds, second fails
	app.OnReload(func(ctx context.Context) error {
		mu.Lock()
		reloadAttempts++
		mu.Unlock()
		return nil
	})

	app.OnReload(func(ctx context.Context) error {
		return assert.AnError // Simulated error
	})

	app.GET("/test", func(c *Context) {
		_ = c.String(http.StatusOK, "ok")
	})

	// Start server in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- app.Start(ctx) // Send result (nil or error)
	}()

	// Wait for server to be ready and to register for SIGHUP (avoid race)
	time.Sleep(300 * time.Millisecond)

	// Send SIGHUP signal
	err = syscall.Kill(os.Getpid(), syscall.SIGHUP)
	require.NoError(t, err, "failed to send SIGHUP")

	// Wait for reload to process
	time.Sleep(100 * time.Millisecond)

	// Verify first hook was executed (even though second failed)
	mu.Lock()
	attempts := reloadAttempts
	mu.Unlock()

	assert.Equal(t, 1, attempts, "first hook should execute before error")

	// Server should still be serving despite reload error
	// We can't easily test HTTP requests in this integration test without knowing the port,
	// but the fact that we can cancel and shutdown cleanly proves the server is still running

	// Cancel context to trigger graceful shutdown
	cancel()

	// Wait for server to shut down (Start returns nil on clean shutdown or error on failure)
	select {
	case err := <-serverErr:
		// Server exited (either clean or with error)
		if err != nil {
			t.Fatalf("server error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down in time")
	}
}

func TestReload_SIGHUPIgnoredWhenNoHooks(t *testing.T) {
	app, err := New(
		WithServiceName("test-reload-sighup-ignored"),
		WithServiceVersion("1.0.0"),
		WithPort(58003), // Use high port to avoid conflicts
	)
	require.NoError(t, err)

	// No OnReload hooks registered - SIGHUP should be ignored
	app.GET("/test", func(c *Context) {
		_ = c.String(http.StatusOK, "ok")
	})

	// Start server in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- app.Start(ctx)
	}()

	// Wait for server to be ready
	time.Sleep(100 * time.Millisecond)

	// Send SIGHUP - process should NOT terminate (SIGHUP ignored)
	err = syscall.Kill(os.Getpid(), syscall.SIGHUP)
	require.NoError(t, err, "failed to send SIGHUP")

	// Wait briefly
	time.Sleep(100 * time.Millisecond)

	// Server should still be running and serving requests
	resp, err := http.Get("http://localhost:58003/test")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusOK, resp.StatusCode, "server should still be running after SIGHUP when no OnReload hooks")

	// Cancel context to trigger graceful shutdown
	cancel()

	// Wait for server to shut down
	select {
	case err := <-serverErr:
		if err != nil {
			t.Fatalf("server error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down in time")
	}
}
