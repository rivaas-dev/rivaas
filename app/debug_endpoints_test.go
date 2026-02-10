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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/router"
)

func TestRegisterDebugEndpoints_pprofDisabledReturnsNilAndNoRoutes(t *testing.T) {
	t.Parallel()

	app, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithDebugEndpoints(), // No WithPprof() -> pprof disabled
	)
	require.NoError(t, err)
	require.NotNil(t, app)

	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	rec := httptest.NewRecorder()
	app.Router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestRegisterDebugEndpoints_customPrefixRegistersUnderPrefix(t *testing.T) {
	t.Parallel()

	app, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithDebugEndpoints(
			WithDebugPrefix("/custom"),
			WithPprof(),
		),
	)
	require.NoError(t, err)
	require.NotNil(t, app)

	req := httptest.NewRequest(http.MethodGet, "/custom/pprof/", nil)
	rec := httptest.NewRecorder()
	app.Router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, strings.Contains(rec.Body.String(), "heap") || strings.Contains(rec.Body.String(), "goroutine"),
		"pprof index should list profiles")

	req = httptest.NewRequest(http.MethodGet, "/custom/pprof/heap", nil)
	rec = httptest.NewRecorder()
	app.Router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRegisterDebugEndpoints_defaultPrefixRegistersPprof(t *testing.T) {
	t.Parallel()

	app, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithDebugEndpoints(WithPprof()),
	)
	require.NoError(t, err)
	require.NotNil(t, app)

	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	rec := httptest.NewRecorder()
	app.Router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, strings.Contains(rec.Body.String(), "heap") || strings.Contains(rec.Body.String(), "goroutine"),
		"pprof index should list profiles")

	req = httptest.NewRequest(http.MethodGet, "/debug/pprof/heap", nil)
	rec = httptest.NewRecorder()
	app.Router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRegisterDebugEndpoints_routeCollisionGETReturnsError(t *testing.T) {
	t.Parallel()

	app, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
	)
	require.NoError(t, err)
	require.NotNil(t, app)

	app.Router().GET("/debug/pprof/", func(c *router.Context) {
		_, writeErr := c.Response.Write([]byte("collision"))
		require.NoError(t, writeErr)
	})
	app.Router().Freeze()

	err = app.registerDebugEndpoints(&debugSettings{
		pprofEnabled: true,
		prefix:       "/debug",
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "route already registered")
	assert.ErrorContains(t, err, "GET")
	assert.ErrorContains(t, err, "/debug/pprof/")
}

func TestRegisterDebugEndpoints_routeCollisionPOSTSymbolReturnsError(t *testing.T) {
	t.Parallel()

	app, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
	)
	require.NoError(t, err)
	require.NotNil(t, app)

	app.Router().POST("/debug/pprof/symbol", func(c *router.Context) {
		_, writeErr := c.Response.Write([]byte("collision"))
		require.NoError(t, writeErr)
	})
	app.Router().Freeze()

	err = app.registerDebugEndpoints(&debugSettings{
		pprofEnabled: true,
		prefix:       "/debug",
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "route already registered")
	assert.ErrorContains(t, err, "POST")
}
