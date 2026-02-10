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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthOptions_defaultPaths(t *testing.T) {
	t.Parallel()

	app, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithHealthEndpoints(), // no opts -> defaultHealthSettings, /healthz, /readyz
	)
	require.NoError(t, err)
	require.NotNil(t, app)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	app.Router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "ok", rec.Body.String())

	req = httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec = httptest.NewRecorder()
	app.Router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestHealthOptions_withHealthPrefix(t *testing.T) {
	t.Parallel()

	app, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithHealthEndpoints(WithHealthPrefix("/_system")),
	)
	require.NoError(t, err)
	require.NotNil(t, app)

	req := httptest.NewRequest(http.MethodGet, "/_system/healthz", nil)
	rec := httptest.NewRecorder()
	app.Router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	req = httptest.NewRequest(http.MethodGet, "/_system/readyz", nil)
	rec = httptest.NewRecorder()
	app.Router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestHealthOptions_withHealthzPathAndReadyzPath(t *testing.T) {
	t.Parallel()

	app, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithHealthEndpoints(
			WithHealthzPath("/live"),
			WithReadyzPath("/ready"),
		),
	)
	require.NoError(t, err)
	require.NotNil(t, app)

	req := httptest.NewRequest(http.MethodGet, "/live", nil)
	rec := httptest.NewRecorder()
	app.Router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	req = httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec = httptest.NewRecorder()
	app.Router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestHealthOptions_withMultipleOptions(t *testing.T) {
	t.Parallel()

	app, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithHealthEndpoints(
			WithHealthPrefix("/_system"),
			WithHealthzPath("/live"),
			WithReadyzPath("/ready"),
			WithHealthTimeout(2*time.Second),
			WithLivenessCheck("ok", func(context.Context) error { return nil }),
			WithReadinessCheck("ok", func(context.Context) error { return nil }),
		),
	)
	require.NoError(t, err)
	require.NotNil(t, app)

	req := httptest.NewRequest(http.MethodGet, "/_system/live", nil)
	rec := httptest.NewRecorder()
	app.Router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	req = httptest.NewRequest(http.MethodGet, "/_system/ready", nil)
	rec = httptest.NewRecorder()
	app.Router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}
