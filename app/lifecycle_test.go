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
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/router/route"
)

func TestOnRoute_fireRouteHookInvokedOnRouteRegistration(t *testing.T) {
	t.Parallel()

	app := MustNew(WithServiceName("test"), WithServiceVersion("1.0.0"))
	var seen []struct{ method, path string }
	var mu sync.Mutex

	require.NoError(t, app.OnRoute(func(rt *route.Route) {
		mu.Lock()
		seen = append(seen, struct{ method, path string }{rt.Method(), rt.Path()})
		mu.Unlock()
	}))

	app.GET("/test", func(c *Context) {})

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, seen, 1)
	assert.Equal(t, "GET", seen[0].method)
	assert.Equal(t, "/test", seen[0].path)
}

func TestOnRoute_multipleRoutesInvokeHookForEach(t *testing.T) {
	t.Parallel()

	app := MustNew(WithServiceName("test"), WithServiceVersion("1.0.0"))
	var seen []struct{ method, path string }
	var mu sync.Mutex

	require.NoError(t, app.OnRoute(func(rt *route.Route) {
		mu.Lock()
		seen = append(seen, struct{ method, path string }{rt.Method(), rt.Path()})
		mu.Unlock()
	}))

	app.GET("/a", func(c *Context) {})
	app.POST("/b", func(c *Context) {})

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, seen, 2)
	assert.Equal(t, "GET", seen[0].method)
	assert.Equal(t, "/a", seen[0].path)
	assert.Equal(t, "POST", seen[1].method)
	assert.Equal(t, "/b", seen[1].path)
}

func TestOnStart_returnsErrorWhenRouterAlreadyFrozen(t *testing.T) {
	t.Parallel()

	app := MustNew(WithServiceName("test"), WithServiceVersion("1.0.0"))
	app.Router().Freeze()

	err := app.OnStart(func(context.Context) error { return nil })
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRouterFrozen), "expected ErrRouterFrozen")
}

func TestOnReady_returnsErrorWhenRouterAlreadyFrozen(t *testing.T) {
	t.Parallel()

	app := MustNew(WithServiceName("test"), WithServiceVersion("1.0.0"))
	app.Router().Freeze()

	err := app.OnReady(func() {})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRouterFrozen), "expected ErrRouterFrozen")
}

func TestOnRoute_returnsErrorWhenRouterAlreadyFrozen(t *testing.T) {
	t.Parallel()

	app := MustNew(WithServiceName("test"), WithServiceVersion("1.0.0"))
	app.Router().Freeze()

	err := app.OnRoute(func(*route.Route) {})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRouterFrozen), "expected ErrRouterFrozen")
}

func TestLifecycleHooks_NilCallbacksReturnError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(*App) error
	}{
		{
			name: "OnStart",
			run:  func(a *App) error { return a.OnStart(nil) },
		},
		{
			name: "OnReady",
			run:  func(a *App) error { return a.OnReady(nil) },
		},
		{
			name: "OnReload",
			run:  func(a *App) error { return a.OnReload(nil) },
		},
		{
			name: "OnShutdown",
			run:  func(a *App) error { return a.OnShutdown(nil) },
		},
		{
			name: "OnStop",
			run:  func(a *App) error { return a.OnStop(nil) },
		},
		{
			name: "OnRoute",
			run:  func(a *App) error { return a.OnRoute(nil) },
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := MustNew(WithServiceName("test"), WithServiceVersion("1.0.0"))
			err := tt.run(a)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "cannot be nil")
		})
	}
}
