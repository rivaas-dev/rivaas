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

package router

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithH2C(t *testing.T) {
	t.Parallel()
	r, err := New(WithH2C(true))
	require.NoError(t, err)
	assert.True(t, r.enableH2C)
}

func TestWithServerTimeouts(t *testing.T) {
	t.Parallel()
	readHeader := 10 * time.Second
	read := 30 * time.Second
	write := 60 * time.Second
	idle := 120 * time.Second
	r, err := New(WithServerTimeouts(
		WithReadHeaderTimeout(readHeader),
		WithReadTimeout(read),
		WithWriteTimeout(write),
		WithIdleTimeout(idle),
	))
	require.NoError(t, err)
	require.NotNil(t, r.serverTimeouts)
	assert.Equal(t, readHeader, r.serverTimeouts.readHeader)
	assert.Equal(t, read, r.serverTimeouts.read)
	assert.Equal(t, write, r.serverTimeouts.write)
	assert.Equal(t, idle, r.serverTimeouts.idle)
}

func TestWithServerTimeouts_PartialOptions(t *testing.T) {
	t.Parallel()
	// Override only read timeout; others should stay at defaults.
	def := defaultServerTimeouts()
	r, err := New(WithServerTimeouts(WithReadTimeout(30 * time.Second)))
	require.NoError(t, err)
	require.NotNil(t, r.serverTimeouts)
	assert.Equal(t, def.readHeader, r.serverTimeouts.readHeader, "readHeader should be default")
	assert.Equal(t, 30*time.Second, r.serverTimeouts.read)
	assert.Equal(t, def.write, r.serverTimeouts.write, "write should be default")
	assert.Equal(t, def.idle, r.serverTimeouts.idle, "idle should be default")
}

func TestNew_WithServerTimeouts_NilOptionReturnsError(t *testing.T) {
	t.Parallel()
	_, err := New(WithServerTimeouts(WithReadTimeout(time.Second), nil))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server timeout option")
	assert.Contains(t, err.Error(), "cannot be nil")
}

func TestWithoutCancellationCheck(t *testing.T) {
	t.Parallel()
	r, err := New(WithoutCancellationCheck())
	require.NoError(t, err)
	assert.False(t, r.checkCancellation)
}

func TestRouteCompilationDefault(t *testing.T) {
	t.Parallel()
	r, err := New()
	require.NoError(t, err)
	assert.False(t, r.useCompiledRoutes, "default should be tree traversal")
}

func TestWithRouteCompilation_OptIn(t *testing.T) {
	t.Parallel()
	r, err := New(WithRouteCompilation(true))
	require.NoError(t, err)
	assert.True(t, r.useCompiledRoutes)
}
