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

	app.OnRoute(func(rt *route.Route) {
		mu.Lock()
		seen = append(seen, struct{ method, path string }{rt.Method(), rt.Path()})
		mu.Unlock()
	})

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

	app.OnRoute(func(rt *route.Route) {
		mu.Lock()
		seen = append(seen, struct{ method, path string }{rt.Method(), rt.Path()})
		mu.Unlock()
	})

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

func TestOnStart_panicsWhenRouterAlreadyFrozen(t *testing.T) {
	t.Parallel()

	app := MustNew(WithServiceName("test"), WithServiceVersion("1.0.0"))
	app.Router().Freeze()

	assert.Panics(t, func() {
		app.OnStart(func(context.Context) error { return nil })
	}, "OnStart should panic when router is already frozen")
}

func TestOnReady_panicsWhenRouterAlreadyFrozen(t *testing.T) {
	t.Parallel()

	app := MustNew(WithServiceName("test"), WithServiceVersion("1.0.0"))
	app.Router().Freeze()

	assert.Panics(t, func() {
		app.OnReady(func() {})
	}, "OnReady should panic when router is already frozen")
}

func TestOnRoute_panicsWhenRouterAlreadyFrozen(t *testing.T) {
	t.Parallel()

	app := MustNew(WithServiceName("test"), WithServiceVersion("1.0.0"))
	app.Router().Freeze()

	assert.Panics(t, func() {
		app.OnRoute(func(*route.Route) {})
	}, "OnRoute should panic when router is already frozen")
}
