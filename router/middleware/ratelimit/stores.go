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

package ratelimit

import (
	"context"
	"sync"
	"time"
)

// tokenBucketEntry represents a token bucket for a single key.
type tokenBucketEntry struct {
	tokens     float64
	lastUpdate time.Time
	mu         sync.Mutex
}

// InMemoryTokenBucketStore implements in-memory token bucket storage.
// This is the default store implementation used by the token bucket rate limiter.
type InMemoryTokenBucketStore struct {
	rate        int // tokens per second
	burst       int // max tokens
	entries     map[string]*tokenBucketEntry
	mu          sync.RWMutex
	cleanup     *time.Ticker
	stopCleanup chan struct{}
}

// NewInMemoryTokenBucketStore creates a new in-memory token bucket store.
// This is exposed to allow custom configuration of the default store.
//
// Example:
//
//	store := ratelimit.NewInMemoryTokenBucketStore(100, 20)
//	r.Use(ratelimit.WithTokenBucket(
//	    ratelimit.TokenBucket{Rate: 100, Burst: 20, Store: store},
//	    ratelimit.CommonOptions{},
//	))
func NewInMemoryTokenBucketStore(rate, burst int) *InMemoryTokenBucketStore {
	store := &InMemoryTokenBucketStore{
		rate:        rate,
		burst:       burst,
		entries:     make(map[string]*tokenBucketEntry),
		stopCleanup: make(chan struct{}),
	}
	// Start cleanup goroutine
	store.cleanup = time.NewTicker(5 * time.Minute)
	go store.cleanupLoop()
	return store
}

// newTokenBucketStore is an internal helper that creates the default store.
func newTokenBucketStore(rate, burst int) *InMemoryTokenBucketStore {
	return NewInMemoryTokenBucketStore(rate, burst)
}

// cleanupLoop periodically removes old entries.
func (s *InMemoryTokenBucketStore) cleanupLoop() {
	for {
		select {
		case <-s.cleanup.C:
			s.mu.Lock()
			// Remove entries older than 1 hour
			cutoff := time.Now().Add(-1 * time.Hour)
			for key, entry := range s.entries {
				entry.mu.Lock()
				if entry.lastUpdate.Before(cutoff) {
					delete(s.entries, key)
				}
				entry.mu.Unlock()
			}
			s.mu.Unlock()
		case <-s.stopCleanup:
			return
		}
	}
}

// Allow checks if a request is allowed and returns remaining tokens and reset time.
// This implements the TokenBucketStore interface.
func (s *InMemoryTokenBucketStore) Allow(key string, now time.Time) (allowed bool, remaining, resetSeconds int) {
	s.mu.RLock()
	entry, exists := s.entries[key]
	s.mu.RUnlock()

	if !exists {
		s.mu.Lock()
		// Double-check after acquiring write lock
		entry, exists = s.entries[key]
		if !exists {
			entry = &tokenBucketEntry{
				tokens:     float64(s.burst),
				lastUpdate: now,
			}
			s.entries[key] = entry
		}
		s.mu.Unlock()
	}

	entry.mu.Lock()
	defer entry.mu.Unlock()

	// Refill tokens based on elapsed time
	elapsed := now.Sub(entry.lastUpdate).Seconds()
	tokensToAdd := elapsed * float64(s.rate)
	entry.tokens = entry.tokens + tokensToAdd
	if entry.tokens > float64(s.burst) {
		entry.tokens = float64(s.burst)
	}
	entry.lastUpdate = now

	// Check if we have tokens
	if entry.tokens >= 1.0 {
		entry.tokens -= 1.0
		remaining = int(entry.tokens)
		resetSeconds = 1 // Reset in 1 second (token bucket refills continuously)
		return true, remaining, resetSeconds
	}

	// No tokens available
	remaining = 0
	// Calculate time until next token is available
	tokensNeeded := 1.0 - entry.tokens
	resetSeconds = int(tokensNeeded / float64(s.rate) * float64(time.Second))
	if resetSeconds < 1 {
		resetSeconds = 1
	}
	return false, remaining, resetSeconds
}

// windowEntry represents a sliding window entry.
type windowEntry struct {
	current     int
	previous    int
	windowStart int64 // Unix timestamp
	mu          sync.Mutex
}

// InMemoryStore implements in-memory sliding window storage.
type InMemoryStore struct {
	entries     map[string]*windowEntry
	mu          sync.RWMutex
	cleanup     *time.Ticker
	stopCleanup chan struct{}
}

// NewInMemoryStore creates a new in-memory sliding window store.
func NewInMemoryStore() *InMemoryStore {
	store := &InMemoryStore{
		entries:     make(map[string]*windowEntry),
		stopCleanup: make(chan struct{}),
	}
	store.cleanup = time.NewTicker(5 * time.Minute)
	go store.cleanupLoop()
	return store
}

// cleanupLoop periodically removes old entries.
func (s *InMemoryStore) cleanupLoop() {
	for {
		select {
		case <-s.cleanup.C:
			s.mu.Lock()
			// Remove entries older than 2 hours
			cutoff := time.Now().Add(-2 * time.Hour).Unix()
			for key, entry := range s.entries {
				entry.mu.Lock()
				if entry.windowStart < cutoff {
					delete(s.entries, key)
				}
				entry.mu.Unlock()
			}
			s.mu.Unlock()
		case <-s.stopCleanup:
			return
		}
	}
}

// GetCounts returns current count, previous count, and window start time.
func (s *InMemoryStore) GetCounts(_ context.Context, key string, window time.Duration) (int, int, int64, error) {
	now := time.Now()
	windowStart := now.Truncate(window).Unix()

	s.mu.RLock()
	entry, exists := s.entries[key]
	s.mu.RUnlock()

	if !exists {
		s.mu.Lock()
		// Double-check
		entry, exists = s.entries[key]
		if !exists {
			entry = &windowEntry{
				current:     0,
				previous:    0,
				windowStart: windowStart,
			}
			s.entries[key] = entry
		}
		s.mu.Unlock()
	}

	entry.mu.Lock()
	defer entry.mu.Unlock()

	// If window has rolled over, shift counts
	if entry.windowStart < windowStart {
		entry.previous = entry.current
		entry.current = 0
		entry.windowStart = windowStart
	}

	return entry.current, entry.previous, entry.windowStart, nil
}

// Incr increments the current window count.
func (s *InMemoryStore) Incr(_ context.Context, key string, window time.Duration) error {
	now := time.Now()
	windowStart := now.Truncate(window).Unix()

	s.mu.RLock()
	entry, exists := s.entries[key]
	s.mu.RUnlock()

	if !exists {
		s.mu.Lock()
		entry, exists = s.entries[key]
		if !exists {
			entry = &windowEntry{
				current:     1,
				previous:    0,
				windowStart: windowStart,
			}
			s.entries[key] = entry
			s.mu.Unlock()
			return nil
		}
		s.mu.Unlock()
	}

	entry.mu.Lock()
	defer entry.mu.Unlock()

	// If window has rolled over, shift counts
	if entry.windowStart < windowStart {
		entry.previous = entry.current
		entry.current = 1
		entry.windowStart = windowStart
	} else {
		entry.current++
	}

	return nil
}
