//go:build test
// +build test

package binding

// ResetCache clears the struct cache (test-only).
// This is useful for testing cache behavior.
func ResetCache() {
	m := make(map[cacheKey]*structInfo)
	structInfoCachePtr.Store(&m)
}

// CacheStats returns cache statistics (test-only).
func CacheStats() int {
	m := structInfoCachePtr.Load()
	if m == nil {
		return 0
	}
	return len(*m)
}
