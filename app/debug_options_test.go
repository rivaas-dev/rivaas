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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithDebugEndpoints_defaults(t *testing.T) {
	t.Parallel()

	app, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithDebugEndpoints(),
	)
	require.NoError(t, err)
	require.NotNil(t, app)

	assert.NotNil(t, app.config.debug)
	assert.True(t, app.config.debug.enabled)
	assert.Equal(t, "/debug", app.config.debug.prefix)
	assert.False(t, app.config.debug.pprofEnabled)
}

func TestWithDebugPrefix(t *testing.T) {
	t.Parallel()

	app, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithDebugEndpoints(WithDebugPrefix("/_internal/debug")),
	)
	require.NoError(t, err)
	require.NotNil(t, app)

	assert.NotNil(t, app.config.debug)
	assert.Equal(t, "/_internal/debug", app.config.debug.prefix)
}

func TestWithPprof(t *testing.T) {
	t.Parallel()

	app, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithDebugEndpoints(WithPprof()),
	)
	require.NoError(t, err)
	require.NotNil(t, app)

	assert.NotNil(t, app.config.debug)
	assert.True(t, app.config.debug.pprofEnabled)
}

func TestWithPprofIf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		condition bool
		wantPprof bool
	}{
		{
			name:      "condition true enables pprof",
			condition: true,
			wantPprof: true,
		},
		{
			name:      "condition false leaves pprof disabled",
			condition: false,
			wantPprof: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			app, err := New(
				WithServiceName("test"),
				WithServiceVersion("1.0.0"),
				WithDebugEndpoints(WithPprofIf(tt.condition)),
			)
			require.NoError(t, err)
			require.NotNil(t, app)

			assert.NotNil(t, app.config.debug)
			assert.Equal(t, tt.wantPprof, app.config.debug.pprofEnabled)
		})
	}
}

func TestWithDebugEndpoints_combinesOptions(t *testing.T) {
	t.Parallel()

	app, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithDebugEndpoints(
			WithDebugPrefix("/_debug"),
			WithPprof(),
		),
	)
	require.NoError(t, err)
	require.NotNil(t, app)

	assert.NotNil(t, app.config.debug)
	assert.Equal(t, "/_debug", app.config.debug.prefix)
	assert.True(t, app.config.debug.pprofEnabled)
}
