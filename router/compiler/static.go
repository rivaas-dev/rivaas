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

// FNV-1a hash constants for inline hashing.
//
// We implement FNV-1a inline instead of using the standard library's hash/fnv
// package for zero-allocation performance in the hot path:
//
//  1. No interface dispatch: fnv.New64a() returns hash.Hash64 interface,
//     and calls to Write()/Sum64() go through interface vtable lookups
//     which prevent inlining. Inline arithmetic is fully inlinable.
//
//  2. No []byte conversion: fnv's Write() method requires []byte, so
//     hashing a string needs []byte(s) conversion (data copy). Inline
//     hashing accesses string bytes directly via index - zero copy.
//
//  3. No string concatenation: to hash "method+path" with fnv, you'd need
//     to concatenate first. Our inline version hashes sequentially - FNV
//     is a streaming hash so the result is mathematically identical.
const (
	fnvOffsetBasis = 14695981039346656037 // FNV-1a 64-bit offset basis
	fnvPrime       = 1099511628211        // FNV-1a 64-bit prime
)

// LookupStatic attempts to find a static route in the hash table.
// After Freeze() is called, this method bypasses the mutex for better performance.
//
// Optimized to avoid allocations:
// - Computes FNV-1a hash inline without creating hash objects
// - Uses pre-computed hash for bloom filter test
// - Skips entirely if no static routes are registered
func (rc *RouteCompiler) LookupStatic(method, path string) *CompiledRoute {
	// Fast path: skip mutex when frozen (data is immutable)
	frozen := rc.frozen.Load()
	if !frozen {
		rc.mu.RLock()
		defer rc.mu.RUnlock()
	}

	// Skip if no static routes
	// Use cached flag when frozen, otherwise check map directly
	if frozen {
		if !rc.hasStatic {
			return nil
		}
	} else {
		if len(rc.staticRoutes) == 0 {
			return nil
		}
	}

	// Compute FNV-1a hash directly without allocations
	// Hash method first, then path (equivalent to hashing method+path)
	hash := uint64(fnvOffsetBasis)
	for i := range len(method) {
		hash ^= uint64(method[i])
		hash *= fnvPrime
	}
	for i := range len(path) {
		hash ^= uint64(path[i])
		hash *= fnvPrime
	}

	// For small route sets, skip bloom filter and check map directly
	// Bloom filter overhead isn't worth it for < 10 routes
	if len(rc.staticRoutes) < 10 {
		return rc.staticRoutes[hash]
	}

	// Bloom filter check using pre-computed hash (avoids recomputing hash)
	if !rc.staticBloom.TestWithPrecomputedHash(hash) {
		return nil // Definitely not present
	}

	return rc.staticRoutes[hash]
}
