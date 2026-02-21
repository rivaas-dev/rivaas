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

package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewInMemoryStore(t *testing.T) {
	t.Parallel()

	store := NewInMemoryStore()
	require.NotNil(t, store)

	ctx := context.Background()
	window := 1 * time.Minute

	// GetCounts for new key returns 0,0 and creates entry
	curr, prev, windowStart, err := store.GetCounts(ctx, "key1", window)
	require.NoError(t, err)
	assert.Equal(t, 0, curr)
	assert.Equal(t, 0, prev)
	assert.Positive(t, windowStart)
}

func TestInMemoryStore_GetCountsAndIncr(t *testing.T) {
	t.Parallel()

	store := NewInMemoryStore()
	ctx := context.Background()
	window := 1 * time.Minute
	key := "test-key"

	// Incr creates entry and returns nil
	err := store.Incr(ctx, key, window)
	require.NoError(t, err)

	// GetCounts returns 1 after one Incr
	curr, prev, windowStart, err := store.GetCounts(ctx, key, window)
	require.NoError(t, err)
	assert.Equal(t, 1, curr)
	assert.Equal(t, 0, prev)
	assert.Positive(t, windowStart)

	// Second Incr in same window
	err = store.Incr(ctx, key, window)
	require.NoError(t, err)
	curr, prev, _, err = store.GetCounts(ctx, key, window)
	require.NoError(t, err)
	assert.Equal(t, 2, curr)
	assert.Equal(t, 0, prev)
}

func TestInMemoryStore_WindowRollover(t *testing.T) {
	t.Parallel()

	store := NewInMemoryStore()
	ctx := context.Background()
	// Use a short window so we can test rollover by truncating to different windows
	window := 2 * time.Second
	key := "rollover-key"

	// Incr once
	err := store.Incr(ctx, key, window)
	require.NoError(t, err)
	curr, prev, _, err := store.GetCounts(ctx, key, window)
	require.NoError(t, err)
	assert.Equal(t, 1, curr)
	assert.Equal(t, 0, prev)

	// After window duration, next GetCounts/Incr uses new window; previous becomes previous
	time.Sleep(2100 * time.Millisecond)
	err = store.Incr(ctx, key, window)
	require.NoError(t, err)
	curr, prev, _, err = store.GetCounts(ctx, key, window)
	require.NoError(t, err)
	// New window: current is 1, previous is 1 (from the earlier Incr)
	assert.Equal(t, 1, curr)
	assert.Equal(t, 1, prev)
}

func TestNewInMemoryTokenBucketStore(t *testing.T) {
	t.Parallel()

	store := NewInMemoryTokenBucketStore(10, 5)
	require.NotNil(t, store)

	// Allow for new key should grant first request
	allowed, remaining, resetSeconds := store.Allow("key1", time.Now())
	assert.True(t, allowed)
	assert.Equal(t, 4, remaining)
	assert.Positive(t, resetSeconds)
}
