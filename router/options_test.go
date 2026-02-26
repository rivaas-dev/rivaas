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
	r, err := New(WithServerTimeouts(readHeader, read, write, idle))
	require.NoError(t, err)
	require.NotNil(t, r.serverTimeouts)
	assert.Equal(t, readHeader, r.serverTimeouts.readHeader)
	assert.Equal(t, read, r.serverTimeouts.read)
	assert.Equal(t, write, r.serverTimeouts.write)
	assert.Equal(t, idle, r.serverTimeouts.idle)
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
