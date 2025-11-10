package router

import "rivaas.dev/router/binding"

// WarmupBindingCache pre-parses struct types to populate the type cache.
// This eliminates first-call reflection overhead for known request types.
// Call this during application startup after defining your structs.
//
// This is a convenience wrapper around binding.WarmupCache that maintains
// backwards compatibility with the router package API.
//
// Example:
//
//	type UserRequest struct { ... }
//	type SearchParams struct { ... }
//
//	router.WarmupBindingCache(
//	    UserRequest{},
//	    SearchParams{},
//	)
func WarmupBindingCache(types ...any) {
	binding.WarmupCache(types...)
}
