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
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/router"
)

func TestRegisterHealthEndpoints_noChecksReturnsOkAndNoContent(t *testing.T) {
	t.Parallel()

	app, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithHealthEndpoints(),
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

func TestRegisterHealthEndpoints_livenessCheckFailsReturns503(t *testing.T) {
	t.Parallel()

	app, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithHealthEndpoints(
			WithLivenessCheck("bad", func(context.Context) error { return errors.New("liveness failed") }),
		),
	)
	require.NoError(t, err)
	require.NotNil(t, app)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	app.Router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestRegisterHealthEndpoints_readinessCheckFailsReturns503(t *testing.T) {
	t.Parallel()

	app, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithHealthEndpoints(
			WithReadinessCheck("bad", func(context.Context) error { return errors.New("readiness failed") }),
		),
	)
	require.NoError(t, err)
	require.NotNil(t, app)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	app.Router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestRegisterHealthEndpoints_customPrefixAndPaths(t *testing.T) {
	t.Parallel()

	app, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithHealthEndpoints(
			WithHealthPrefix("/_system"),
			WithHealthzPath("/live"),
			WithReadyzPath("/ready"),
		),
	)
	require.NoError(t, err)
	require.NotNil(t, app)

	req := httptest.NewRequest(http.MethodGet, "/_system/live", nil)
	rec := httptest.NewRecorder()
	app.Router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "ok", rec.Body.String())

	req = httptest.NewRequest(http.MethodGet, "/_system/ready", nil)
	rec = httptest.NewRecorder()
	app.Router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestRegisterHealthEndpoints_routeCollisionReturnsError(t *testing.T) {
	t.Parallel()

	app, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
	)
	require.NoError(t, err)
	require.NotNil(t, app)

	app.Router().GET("/healthz", func(c *router.Context) {
		_, writeErr := c.Response.Write([]byte("collision"))
		require.NoError(t, writeErr)
	})
	app.Router().Freeze()

	s := defaultHealthSettings()
	err = app.registerHealthEndpoints(s)
	require.Error(t, err)
	assert.ErrorContains(t, err, "route already registered")
	assert.ErrorContains(t, err, "GET")
	assert.ErrorContains(t, err, "healthz")
}

func TestRunChecks_allPassReturnsEmptyMap(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	checks := map[string]CheckFunc{
		"a": func(context.Context) error { return nil },
		"b": func(context.Context) error { return nil },
	}
	failures := runChecks(ctx, checks, time.Second)
	assert.Empty(t, failures)
}

func TestRunChecks_oneFailureReturnsFailureMap(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	errBad := errors.New("check b failed")
	checks := map[string]CheckFunc{
		"a": func(context.Context) error { return nil },
		"b": func(context.Context) error { return errBad },
	}
	failures := runChecks(ctx, checks, time.Second)
	require.Len(t, failures, 1)
	assert.Equal(t, "check b failed", failures["b"])
}

func TestRunChecks_multipleFailuresReturnsAll(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	checks := map[string]CheckFunc{
		"a": func(context.Context) error { return errors.New("a failed") },
		"b": func(context.Context) error { return errors.New("b failed") },
	}
	failures := runChecks(ctx, checks, time.Second)
	require.Len(t, failures, 2)
	assert.Equal(t, "a failed", failures["a"])
	assert.Equal(t, "b failed", failures["b"])
}

func TestRunChecks_timeoutShortCircuits(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	checks := map[string]CheckFunc{
		"slow": func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(2 * time.Second):
				return nil
			}
		},
	}
	failures := runChecks(ctx, checks, 50*time.Millisecond)
	require.Len(t, failures, 1)
	assert.Contains(t, failures["slow"], "context deadline exceeded")
}
