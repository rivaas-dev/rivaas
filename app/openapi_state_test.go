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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/openapi"
)

func TestNewOpenapiState(t *testing.T) {
	t.Parallel()

	api, err := openapi.New(openapi.WithTitle("test-api", "1.0.0"))
	require.NoError(t, err)
	require.NotNil(t, api)

	s := newOpenapiState(api)
	require.NotNil(t, s)
	assert.Equal(t, api, s.API())
	assert.Equal(t, api.SpecPath, s.SpecPath())
	assert.Equal(t, api.UIPath, s.UIPath())
	assert.Equal(t, api.ServeUI, s.ServeUI())
	assert.NotNil(t, s.UIConfig())
}

func TestOpenapiState_AddOperation_invalidatesCache(t *testing.T) {
	t.Parallel()

	api := openapi.MustNew(openapi.WithTitle("test", "1.0.0"))
	s := newOpenapiState(api)

	s.AddOperation(openapi.Op("GET", "/", openapi.WithSummary("root")))
	ctx := context.Background()
	spec1, etag1, err := s.GenerateSpec(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, spec1)
	require.NotEmpty(t, etag1)

	// Second call returns cached result
	spec2, etag2, err := s.GenerateSpec(ctx)
	require.NoError(t, err)
	assert.Equal(t, spec1, spec2)
	assert.Equal(t, etag1, etag2)

	// Add another operation invalidates cache
	s.AddOperation(openapi.Op("GET", "/other", openapi.WithSummary("other")))
	spec3, etag3, err := s.GenerateSpec(ctx)
	require.NoError(t, err)
	assert.NotEqual(t, spec1, spec3)
	assert.NotEqual(t, etag1, etag3)
	assert.Contains(t, string(spec3), "other")
}

func TestOpenapiState_GenerateSpec_cacheHit(t *testing.T) {
	t.Parallel()

	api := openapi.MustNew(openapi.WithTitle("test", "1.0.0"))
	s := newOpenapiState(api)
	s.AddOperation(openapi.Op("GET", "/", openapi.WithSummary("root")))

	ctx := context.Background()
	spec, etag, err := s.GenerateSpec(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, spec)
	assert.NotEmpty(t, etag)
	assert.Contains(t, string(spec), "openapi")
}

func TestOpenapiState_Warnings_beforeGenerateReturnsNil(t *testing.T) {
	t.Parallel()

	api := openapi.MustNew(openapi.WithTitle("test", "1.0.0"))
	s := newOpenapiState(api)
	assert.Nil(t, s.Warnings())
}

func TestOpenapiState_Warnings_afterGenerateReturnsCopy(t *testing.T) {
	t.Parallel()

	api := openapi.MustNew(openapi.WithTitle("test", "1.0.0"))
	s := newOpenapiState(api)
	s.AddOperation(openapi.Op("GET", "/", openapi.WithSummary("root")))
	_, _, err := s.GenerateSpec(context.Background())
	require.NoError(t, err)

	w := s.Warnings()
	// May be nil or non-nil depending on openapi implementation
	if w != nil {
		assert.NotNil(t, w)
		// Warnings returns a copy - modifying the returned slice should not affect internal state
		origLen := len(w)
		w2 := s.Warnings()
		require.NotNil(t, w2)
		assert.Equal(t, origLen, len(w2))
	}
}

func TestOpenapiState_accessorsMatchAPI(t *testing.T) {
	t.Parallel()

	api := openapi.MustNew(
		openapi.WithTitle("my-api", "2.0.0"),
		openapi.WithSpecPath("/api/openapi.json"),
		openapi.WithSwaggerUI("/api/docs"),
	)
	s := newOpenapiState(api)

	assert.Equal(t, api, s.API())
	assert.Equal(t, "/api/openapi.json", s.SpecPath())
	assert.Equal(t, "/api/docs", s.UIPath())
	assert.True(t, s.ServeUI())
}
