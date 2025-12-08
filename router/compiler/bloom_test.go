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

package compiler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewBloomFilter tests bloom filter creation.
func TestNewBloomFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		size         uint64
		numHashFuncs int
		wantPanic    bool
	}{
		{
			name:         "standard size",
			size:         1000,
			numHashFuncs: 3,
			wantPanic:    false,
		},
		{
			name:         "small size",
			size:         10,
			numHashFuncs: 2,
			wantPanic:    false,
		},
		{
			name:         "large size",
			size:         100000,
			numHashFuncs: 5,
			wantPanic:    false,
		},
		{
			name:         "single hash function",
			size:         100,
			numHashFuncs: 1,
			wantPanic:    false,
		},
		{
			name:         "zero hash functions",
			size:         100,
			numHashFuncs: 0,
			wantPanic:    false, // Should create empty seeds slice
		},
		{
			name:         "zero size",
			size:         0,
			numHashFuncs: 3,
			wantPanic:    false, // Should handle gracefully
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.wantPanic {
				assert.Panics(t, func() {
					NewBloomFilter(tt.size, tt.numHashFuncs)
				})

				return
			}

			bf := NewBloomFilter(tt.size, tt.numHashFuncs)
			require.NotNil(t, bf, "bloom filter should not be nil")
			assert.Equal(t, tt.size, bf.size)
			assert.Len(t, bf.seeds, tt.numHashFuncs)
		})
	}
}

// TestBloomFilter_Add tests adding items to the bloom filter.
func TestBloomFilter_Add(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "regular string",
			data: []byte("test"),
		},
		{
			name: "empty data",
			data: []byte{},
		},
		{
			name: "nil data",
			data: nil,
		},
		{
			name: "long string",
			data: []byte("this is a very long string that should still work correctly with the bloom filter"),
		},
		{
			name: "binary data",
			data: []byte{0x00, 0x01, 0x02, 0xFF, 0xFE},
		},
		{
			name: "unicode data",
			data: []byte("日本語テスト"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			bf := NewBloomFilter(1000, 3)

			// Should not panic
			assert.NotPanics(t, func() {
				bf.Add(tt.data)
			})

			// After adding, Test should return true
			assert.True(t, bf.Test(tt.data), "added item should test positive")
		})
	}
}

// TestBloomFilter_Test tests membership testing.
func TestBloomFilter_Test(t *testing.T) {
	t.Parallel()

	bf := NewBloomFilter(1000, 3)

	// Add some items
	items := [][]byte{
		[]byte("GET/users"),
		[]byte("POST/users"),
		[]byte("GET/posts"),
	}

	for _, item := range items {
		bf.Add(item)
	}

	tests := []struct {
		name       string
		data       []byte
		wantResult bool
	}{
		{
			name:       "existing item 1",
			data:       []byte("GET/users"),
			wantResult: true,
		},
		{
			name:       "existing item 2",
			data:       []byte("POST/users"),
			wantResult: true,
		},
		{
			name:       "existing item 3",
			data:       []byte("GET/posts"),
			wantResult: true,
		},
		{
			name:       "empty data after add",
			data:       []byte{},
			wantResult: false, // Empty was not added
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := bf.Test(tt.data)
			assert.Equal(t, tt.wantResult, result)
		})
	}
}

// TestBloomFilter_FalsePositiveRate tests the false positive behavior.
func TestBloomFilter_FalsePositiveRate(t *testing.T) {
	t.Parallel()

	// Use a smaller filter to increase false positive rate for testing
	bf := NewBloomFilter(100, 3)

	// Add items
	addedCount := 50
	for i := range addedCount {
		bf.Add([]byte("/route" + string(rune('0'+i))))
	}

	// Test that all added items return true
	for i := range addedCount {
		assert.True(t, bf.Test([]byte("/route"+string(rune('0'+i)))), "added items should always test positive")
	}

	// Test items not added - count false positives
	testCount := 100
	falsePositives := 0
	for i := range testCount {
		if bf.Test([]byte("/nonexistent" + string(rune('0'+i)))) {
			falsePositives++
		}
	}

	// With a small filter (100 bits) and 50 items, we expect some false positives
	// but not all should be false positives
	assert.Less(t, falsePositives, testCount, "should have some true negatives")

	// Log the false positive rate for informational purposes
	t.Logf("False positive rate: %.2f%% (%d/%d)", float64(falsePositives)/float64(testCount)*100, falsePositives, testCount)
}

// TestBloomFilter_EmptyFilter tests behavior of an empty filter.
func TestBloomFilter_EmptyFilter(t *testing.T) {
	t.Parallel()

	bf := NewBloomFilter(1000, 3)

	// Test on empty filter should return false
	assert.False(t, bf.Test([]byte("anything")), "empty filter should return false for any query")
	assert.False(t, bf.Test([]byte("")), "empty filter should return false for empty query")
	assert.False(t, bf.Test(nil), "empty filter should return false for nil query")
}

// TestBloomFilter_HashConsistency tests that the same data always produces the same result.
func TestBloomFilter_HashConsistency(t *testing.T) {
	t.Parallel()

	bf := NewBloomFilter(1000, 3)
	data := []byte("consistent-test-data")

	bf.Add(data)

	// Multiple tests should always return true
	for range 100 {
		assert.True(t, bf.Test(data), "consistent data should always test positive")
	}
}

// TestBloomFilter_ZeroSizeEdgeCase tests behavior with zero size.
// NOTE: Zero size is an invalid configuration that causes division by zero.
// Callers should always use size > 0. This test documents the current behavior.
func TestBloomFilter_ZeroSizeEdgeCase(t *testing.T) {
	t.Parallel()

	// Zero size creates a filter with at least 1 element in bits slice
	// due to (size+63)/64 calculation, which gives 1 when size=0.
	// However, the modulo operation in hashWithSeed causes division by zero.
	bf := NewBloomFilter(0, 3)

	// With size=0, operations panic due to division by zero in hashWithSeed
	// This is expected behavior - callers must use size > 0
	assert.Panics(t, func() {
		bf.Add([]byte("test"))
	}, "zero size should panic on Add due to division by zero")

	assert.Panics(t, func() {
		bf.Test([]byte("test"))
	}, "zero size should panic on Test due to division by zero")
}

// TestBloomFilter_MultipleHashFunctions tests different numbers of hash functions.
func TestBloomFilter_MultipleHashFunctions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		numHashFuncs int
	}{
		{"1 hash function", 1},
		{"2 hash functions", 2},
		{"3 hash functions", 3},
		{"5 hash functions", 5},
		{"10 hash functions", 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			bf := NewBloomFilter(1000, tt.numHashFuncs)

			// Add and test
			data := []byte("test-data")
			bf.Add(data)
			assert.True(t, bf.Test(data), "added data should test positive")
		})
	}
}
