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

// LookupStatic attempts to find a static route in the hash table
func (rc *RouteCompiler) LookupStatic(method, path string) *CompiledRoute {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	// Bloom filter check first (negative lookup)
	key := method + path
	if !rc.staticBloom.Test([]byte(key)) {
		return nil // Definitely not present
	}

	// Hash lookup
	h := fnv.New64a()
	h.Write([]byte(key))
	hash := h.Sum64()
	return rc.staticRoutes[hash]
}
