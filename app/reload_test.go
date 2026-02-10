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

package app

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOnReload_Sequential(t *testing.T) {
	t.Parallel()

	app, err := New(
		WithServiceName("test-reload-sequential"),
		WithServiceVersion("1.0.0"),
	)
	require.NoError(t, err)

	var executionOrder []int
	var mu sync.Mutex

	app.OnReload(func(ctx context.Context) error {
		mu.Lock()
		executionOrder = append(executionOrder, 1)
		mu.Unlock()
		return nil
	})

	app.OnReload(func(ctx context.Context) error {
		mu.Lock()
		executionOrder = append(executionOrder, 2)
		mu.Unlock()
		return nil
	})

	app.OnReload(func(ctx context.Context) error {
		mu.Lock()
		executionOrder = append(executionOrder, 3)
		mu.Unlock()
		return nil
	})

	ctx := context.Background()
	err = app.Reload(ctx)
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []int{1, 2, 3}, executionOrder, "hooks should execute in registration order")
}

func TestOnReload_StopsOnError(t *testing.T) {
	t.Parallel()

	app, err := New(
		WithServiceName("test-reload-stops-on-error"),
		WithServiceVersion("1.0.0"),
	)
	require.NoError(t, err)

	var executed []int
	var mu sync.Mutex

	app.OnReload(func(ctx context.Context) error {
		mu.Lock()
		executed = append(executed, 1)
		mu.Unlock()
		return nil
	})

	app.OnReload(func(ctx context.Context) error {
		mu.Lock()
		executed = append(executed, 2)
		mu.Unlock()
		return errors.New("hook 2 failed")
	})

	app.OnReload(func(ctx context.Context) error {
		mu.Lock()
		executed = append(executed, 3)
		mu.Unlock()
		return nil
	})

	ctx := context.Background()
	err = app.Reload(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OnReload hook 1 failed")
	assert.Contains(t, err.Error(), "hook 2 failed")

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []int{1, 2}, executed, "subsequent hooks should not execute after error")
}

func TestOnReload_ServerContinuesOnError(t *testing.T) {
	t.Parallel()

	app, err := New(
		WithServiceName("test-reload-server-continues"),
		WithServiceVersion("1.0.0"),
	)
	require.NoError(t, err)

	app.OnReload(func(ctx context.Context) error {
		return errors.New("reload failed")
	})

	app.GET("/test", func(c *Context) {
		_ = c.String(http.StatusOK, "ok") //nolint:errcheck // Test code
	})

	// Simulate reload failure
	ctx := context.Background()
	err = app.Reload(ctx)
	require.Error(t, err)

	// Server should still be able to handle requests
	resp, err := app.TestJSON(http.MethodGet, "/test", nil)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // Test code

	assert.Equal(t, http.StatusOK, resp.StatusCode, "server should continue serving after reload error")
}

func TestOnReload_PanicIfFrozen(t *testing.T) {
	t.Parallel()

	app, err := New(
		WithServiceName("test-reload-panic-frozen"),
		WithServiceVersion("1.0.0"),
	)
	require.NoError(t, err)

	// Freeze the router
	app.router.Freeze()

	// Registering OnReload after freeze should panic
	assert.Panics(t, func() {
		app.OnReload(func(ctx context.Context) error {
			return nil
		})
	}, "should panic when registering OnReload after router is frozen")
}

func TestReload_Serialized(t *testing.T) {
	t.Parallel()

	app, err := New(
		WithServiceName("test-reload-serialized"),
		WithServiceVersion("1.0.0"),
	)
	require.NoError(t, err)

	var activeReloads int32
	var maxConcurrent int32
	var mu sync.Mutex

	app.OnReload(func(ctx context.Context) error {
		mu.Lock()
		activeReloads++
		if activeReloads > maxConcurrent {
			maxConcurrent = activeReloads
		}
		mu.Unlock()

		// Simulate some work
		time.Sleep(50 * time.Millisecond)

		mu.Lock()
		activeReloads--
		mu.Unlock()

		return nil
	})

	ctx := context.Background()
	var wg sync.WaitGroup

	// Launch multiple concurrent reloads
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = app.Reload(ctx) //nolint:errcheck // Test is checking serialization, not error handling
		}()
	}

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, int32(1), maxConcurrent, "only one reload should execute at a time (serialized)")
}

func TestReload_Programmatic(t *testing.T) {
	t.Parallel()

	// Test that Reload() works programmatically
	app, err := New(
		WithServiceName("test-reload-programmatic"),
		WithServiceVersion("1.0.0"),
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

	ctx := context.Background()

	// Call Reload() programmatically multiple times
	for i := 0; i < 3; i++ {
		err = app.Reload(ctx)
		require.NoError(t, err)
	}

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 3, reloadCount, "reload should work programmatically")
}

func TestOnReload_NoHooksRegistered(t *testing.T) {
	t.Parallel()

	app, err := New(
		WithServiceName("test-reload-no-hooks"),
		WithServiceVersion("1.0.0"),
	)
	require.NoError(t, err)

	// Reload with no hooks registered should succeed
	ctx := context.Background()
	err = app.Reload(ctx)
	require.NoError(t, err)
}

func TestOnReload_ContextCancellation(t *testing.T) {
	t.Parallel()

	app, err := New(
		WithServiceName("test-reload-context-cancel"),
		WithServiceVersion("1.0.0"),
	)
	require.NoError(t, err)

	app.OnReload(func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			return nil
		}
	})

	// Create a context that's already canceled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = app.Reload(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestOnReload_MultipleReloads(t *testing.T) {
	t.Parallel()

	app, err := New(
		WithServiceName("test-reload-multiple"),
		WithServiceVersion("1.0.0"),
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

	ctx := context.Background()

	// Perform multiple reloads
	for i := 0; i < 10; i++ {
		err = app.Reload(ctx)
		require.NoError(t, err)
	}

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 10, reloadCount, "all reloads should execute")
}
