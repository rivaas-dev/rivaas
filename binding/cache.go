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

package binding

import (
	"fmt"
	"maps"
	"reflect"
	"sync"
	"sync/atomic"
)

var (
	// RCU pattern: atomic pointer to immutable map
	structInfoCachePtr atomic.Pointer[map[cacheKey]*structInfo]

	// Write-side lock (only for cache updates)
	structInfoCacheMu sync.Mutex
)

func init() {
	// Initialize with empty map
	m := make(map[cacheKey]*structInfo)
	structInfoCachePtr.Store(&m)
}

// cacheKey is the key for the struct cache.
type cacheKey struct {
	typ reflect.Type
	tag string
}

// getStructInfo retrieves or parses struct information from the cache.
// It uses a read-copy-update pattern for concurrent access.
//
// The function is safe for concurrent use. Multiple goroutines calling this
// with the same type+tag will only parse once (double-check locking).
//
// Parameters:
//   - typ: The struct type to parse (must be reflect.Struct, not pointer)
//   - tag: The struct tag name to use for field binding (e.g., "json", "query")
//
// Returns cached metadata containing field information and validation rules.
func getStructInfo(typ reflect.Type, tag string) *structInfo {
	// Defensive checks: validate input
	if typ == nil {
		panic("binding: getStructInfo called with nil type")
	}
	if tag == "" {
		panic("binding: getStructInfo called with empty tag")
	}

	// Normalize: unwrap pointer to get struct type
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	if typ.Kind() != reflect.Struct {
		panic(fmt.Sprintf("binding: getStructInfo expects struct, got %s", typ.Kind()))
	}

	key := cacheKey{typ: typ, tag: tag}

	// Check cache first: lock-free read from current map
	m := structInfoCachePtr.Load()
	if si, ok := (*m)[key]; ok {
		return si
	}

	// Cache miss: acquire write lock
	structInfoCacheMu.Lock()
	defer structInfoCacheMu.Unlock()

	// Double-check: another goroutine might have populated it
	m = structInfoCachePtr.Load()
	if si, ok := (*m)[key]; ok {
		return si
	}

	// Parse struct info
	si := parseStructInfo(typ, tag)

	// Copy-on-write: create new map with added entry
	newMap := make(map[cacheKey]*structInfo, len(*m)+1)
	maps.Copy(newMap, *m)
	newMap[key] = si

	// Atomic swap (readers instantly see new map)
	structInfoCachePtr.Store(&newMap)

	return si
}

// WarmupCache pre-parses struct types to populate the type cache.
// Call this during application startup after defining your structs to populate
// the cache for known request types.
//
// Invalid types are silently skipped. Use MustWarmupCache to panic on errors.
//
// Example:
//
//	type UserRequest struct { ... }
//	type SearchParams struct { ... }
//
//	binding.WarmupCache(
//	    UserRequest{},
//	    SearchParams{},
//	)
//
// Parameters:
//   - types: Variadic list of struct instances to warm up
func WarmupCache(types ...any) {
	tags := []string{TagJSON, TagQuery, TagPath, TagForm, TagCookie, TagHeader}
	for _, t := range types {
		typ := reflect.TypeOf(t)
		if typ == nil {
			continue
		}
		if typ.Kind() == reflect.Ptr {
			typ = typ.Elem()
		}
		if typ.Kind() != reflect.Struct {
			continue
		}

		// Parse for all common tag types
		for _, tag := range tags {
			getStructInfo(typ, tag)
		}
	}
}

// MustWarmupCache is like WarmupCache but panics on invalid types.
// Use during application startup to validate struct tags at startup.
//
// Example:
//
//	func init() {
//	    binding.MustWarmupCache(
//	        UserRequest{},
//	        SearchParams{},
//	    )
//	}
//
// Parameters:
//   - types: Variadic list of struct instances to warm up
//
// Panics if any type is invalid or has invalid struct tags.
func MustWarmupCache(types ...any) {
	tags := []string{TagJSON, TagQuery, TagPath, TagForm, TagCookie, TagHeader}
	for _, t := range types {
		typ := reflect.TypeOf(t)
		if typ == nil {
			panic("binding: MustWarmupCache called with nil type")
		}
		if typ.Kind() == reflect.Ptr {
			typ = typ.Elem()
		}
		if typ.Kind() != reflect.Struct {
			panic(fmt.Sprintf("binding: MustWarmupCache expects struct, got %s", typ.Kind()))
		}

		// Parse for all common tag types (will panic on invalid tags)
		for _, tag := range tags {
			getStructInfo(typ, tag)
		}
	}
}
