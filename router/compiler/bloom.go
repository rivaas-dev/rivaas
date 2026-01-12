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

import "hash/fnv"

// BloomFilter provides a simple bloom filter implementation for negative lookups.
// A bloom filter is a probabilistic data structure that can tell you:
// - "Definitely NOT in the set" (100% accurate)
// - "Possibly in the set" (may have false positives)
//
// Use case in routing: Filter out paths that definitely don't exist
// before checking the route map.
//
// How it works:
// 1. Hash the input with multiple hash functions (using different seeds)
// 2. Set bits at the hash positions when adding elements
// 3. Check if all bits are set when testing membership
// 4. If any bit is unset → element definitely not in set (true negative)
// 5. If all bits are set → element might be in set (check actual map)
//
// Uses multiple hash functions (typically 3)
// Uses bit array for compact storage
//
// Implementation using FNV-1a hash with different seeds
type BloomFilter struct {
	bits  []uint64 // Bit array (each uint64 holds 64 bits)
	size  uint64   // Total number of bits
	seeds []uint64 // Hash seeds for multiple hash functions
}

// NewBloomFilter creates a new bloom filter with the specified size and hash functions.
// Uses FNV-1a hash with different seeds.
func NewBloomFilter(size uint64, numHashFuncs int) *BloomFilter {
	bf := &BloomFilter{
		bits:  make([]uint64, (size+63)/64), // Round up to nearest 64-bit boundary
		size:  size,
		seeds: make([]uint64, numHashFuncs),
	}

	// Initialize seeds for hash functions
	for i := range numHashFuncs {
		//nolint:gosec // G115: numHashFuncs is small (typically < 10), overflow impossible
		bf.seeds[i] = uint64(i + 1)
	}

	return bf
}

// hashWithSeed applies a seed to a pre-computed base hash.
// The seed is XORed with the base hash to create different hash functions.
// This avoids repeatedly creating hash.Hash instances.
func (bf *BloomFilter) hashWithSeed(baseHash, seed uint64) uint64 {
	// XOR with seed to create different hash functions for bloom filter
	return (baseHash ^ seed) % bf.size
}

// Add adds an element to the bloom filter
func (bf *BloomFilter) Add(data []byte) {
	// Compute base hash once, then apply all seeds
	h := fnv.New64a()
	h.Write(data)
	baseHash := h.Sum64()

	for _, seed := range bf.seeds {
		pos := bf.hashWithSeed(baseHash, seed)
		bf.bits[pos/64] |= 1 << (pos % 64)
	}
}

// Test checks if an element might be in the bloom filter
//
// Uses early-exit loop for handling of miss cases (common in routing).
// Bloom filters are most valuable when they can reject non-existent routes,
// so early exit on first failed bit check is important.
func (bf *BloomFilter) Test(data []byte) bool {
	// Compute base hash once, then apply all seeds
	h := fnv.New64a()
	h.Write(data)
	baseHash := h.Sum64()

	for _, seed := range bf.seeds {
		pos := bf.hashWithSeed(baseHash, seed)
		if bf.bits[pos/64]&(1<<(pos%64)) == 0 {
			return false // Early exit - definitely not present
		}
	}

	return true
}

// TestWithPrecomputedHash checks if an element might be in the bloom filter
// using a pre-computed FNV-1a hash. This avoids recomputing the hash when
// the caller has already computed it.
//
// Uses early-exit loop for handling of miss cases (common in routing).
func (bf *BloomFilter) TestWithPrecomputedHash(baseHash uint64) bool {
	for _, seed := range bf.seeds {
		pos := bf.hashWithSeed(baseHash, seed)
		if bf.bits[pos/64]&(1<<(pos%64)) == 0 {
			return false // Early exit - definitely not present
		}
	}

	return true
}
