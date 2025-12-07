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

package router

import (
	"net/http"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"

	"rivaas.dev/router/version"
)

// TestAtomicFieldAlignment verifies that atomic fields are properly aligned
// for atomic operations on all supported platforms (64-bit architectures).
//
// Background:
// Atomic operations on uint64 and unsafe.Pointer require proper alignment:
//   - On 64-bit platforms: 8-byte alignment (guaranteed by Go runtime)
//   - On 32-bit platforms: 8-byte alignment (NOT guaranteed, requires first field)
//
// Our approach:
//   - Panic on non-64-bit platforms (see routes.go init())
//   - Verify alignment on 64-bit platforms (this test)
//   - Place atomic fields first in structs (best practice)
//
// References:
//   - https://pkg.go.dev/sync/atomic#pkg-note-BUG
//   - https://go.dev/ref/spec#Size_and_alignment_guarantees
func TestAtomicFieldAlignment(t *testing.T) {
	t.Parallel()

	// Verify we're on a 64-bit platform
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Skip("Skipping alignment test: requires 64-bit platform")
	}

	t.Run("atomicRouteTree.trees alignment", func(t *testing.T) {
		t.Parallel()
		var tree atomicRouteTree
		treesOffset := unsafe.Offsetof(tree.trees)

		// trees should be at offset 0 (first field)
		assert.Equal(t, uintptr(0), treesOffset, "atomicRouteTree.trees must be first field for proper alignment")

		// trees pointer should be 8-byte aligned
		treeAddr := uintptr(unsafe.Pointer(&tree.trees))
		assert.Equal(t, uintptr(0), treeAddr%8, "atomicRouteTree.trees is not 8-byte aligned: address=%x (mod 8 = %d)", treeAddr, treeAddr%8)
	})

	t.Run("atomicRouteTree.version alignment", func(t *testing.T) {
		t.Parallel()
		var tree atomicRouteTree
		versionOffset := unsafe.Offsetof(tree.version)

		// version should be 8-byte aligned (offset must be multiple of 8)
		assert.Equal(t, uintptr(0), versionOffset%8, "atomicRouteTree.version is not 8-byte aligned: offset=%d (mod 8 = %d)",
			versionOffset, versionOffset%8)

		// Verify actual address is 8-byte aligned
		versionAddr := uintptr(unsafe.Pointer(&tree.version))
		assert.Equal(t, uintptr(0), versionAddr%8, "atomicRouteTree.version address is not 8-byte aligned: address=%x (mod 8 = %d)",
			versionAddr, versionAddr%8)
	})

	t.Run("atomicVersionTrees.trees alignment", func(t *testing.T) {
		t.Parallel()
		var vt atomicVersionTrees
		treesOffset := unsafe.Offsetof(vt.trees)

		// trees should be at offset 0 (first and only field)
		assert.Equal(t, uintptr(0), treesOffset, "atomicVersionTrees.trees must be first field")

		// trees pointer should be 8-byte aligned
		treeAddr := uintptr(unsafe.Pointer(&vt.trees))
		assert.Equal(t, uintptr(0), treeAddr%8, "atomicVersionTrees.trees is not 8-byte aligned: address=%x (mod 8 = %d)",
			treeAddr, treeAddr%8)
	})

	t.Run("Router.routeTree alignment", func(t *testing.T) {
		t.Parallel()
		var r Router
		routeTreeOffset := unsafe.Offsetof(r.routeTree)

		// routeTree should be at offset 0 (first field) for optimal alignment
		assert.Equal(t, uintptr(0), routeTreeOffset, "Router.routeTree should be first field for alignment")

		// Verify the atomic fields within routeTree are properly aligned
		treesAddr := uintptr(unsafe.Pointer(&r.routeTree.trees))
		assert.Equal(t, uintptr(0), treesAddr%8, "Router.routeTree.trees is not 8-byte aligned: address=%x (mod 8 = %d)",
			treesAddr, treesAddr%8)

		versionAddr := uintptr(unsafe.Pointer(&r.routeTree.version))
		assert.Equal(t, uintptr(0), versionAddr%8, "Router.routeTree.version is not 8-byte aligned: address=%x (mod 8 = %d)",
			versionAddr, versionAddr%8)
	})

	t.Run("Router.versionTrees alignment", func(t *testing.T) {
		t.Parallel()
		var r Router
		versionTreesOffset := unsafe.Offsetof(r.versionTrees)

		// versionTrees should be 8-byte aligned (offset must be multiple of 8)
		assert.Equal(t, uintptr(0), versionTreesOffset%8, "Router.versionTrees is not 8-byte aligned: offset=%d (mod 8 = %d)",
			versionTreesOffset, versionTreesOffset%8)

		// Verify actual address is 8-byte aligned
		versionTreesAddr := uintptr(unsafe.Pointer(&r.versionTrees.trees))
		assert.Equal(t, uintptr(0), versionTreesAddr%8, "Router.versionTrees.trees is not 8-byte aligned: address=%x (mod 8 = %d)",
			versionTreesAddr, versionTreesAddr%8)
	})
}

// TestStructSizes documents the size and alignment of key structs
// to catch unintended changes during refactoring.
func TestStructSizes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		size         uintptr
		expectedSize uintptr
		maxSize      uintptr // Warn if size exceeds this
	}{
		{
			name:         "atomicRouteTree",
			size:         unsafe.Sizeof(atomicRouteTree{}),
			expectedSize: 64, // 8 (trees) + 8 (version) + 24 (routes slice) + 24 (RWMutex)
			maxSize:      80,
		},
		{
			name:         "atomicVersionTrees",
			size:         unsafe.Sizeof(atomicVersionTrees{}),
			expectedSize: 8, // Just unsafe.Pointer
			maxSize:      16,
		},
		{
			name:         "Router",
			size:         unsafe.Sizeof(Router{}),
			expectedSize: 0,   // Not checking exact size, just documenting
			maxSize:      450, // Warn if Router grows beyond reasonable size (includes deferred registration fields)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			t.Logf("%s size: %d bytes", tt.name, tt.size)

			if tt.expectedSize > 0 && tt.size != tt.expectedSize {
				t.Logf("WARNING: %s size changed from %d to %d bytes (delta: %+d)",
					tt.name, tt.expectedSize, tt.size, int(tt.size)-int(tt.expectedSize))
			}

			if tt.size > tt.maxSize {
				assert.Failf(t, "size exceeds maximum", "%s size (%d bytes) exceeds maximum (%d bytes)",
					tt.name, tt.size, tt.maxSize)
			}
		})
	}
}

// TestAtomicOperationsSafety verifies that atomic operations work correctly
// on the atomic fields without panics or race conditions.
func TestAtomicOperationsSafety(t *testing.T) {
	t.Parallel()

	// This test ensures atomic operations don't panic due to misalignment

	t.Run("atomicRouteTree operations", func(t *testing.T) {
		t.Parallel()
		r := MustNew()

		// Register a route (triggers atomic operations)
		r.GET("/test", func(c *Context) {
			c.String(http.StatusOK, "OK")
		})

		// The fact that this doesn't panic means alignment is correct
		t.Log("Atomic operations on routeTree completed successfully")
	})

	t.Run("atomicVersionTrees operations", func(t *testing.T) {
		t.Parallel()
		r := MustNew(WithVersioning(
			version.WithQueryDetection("version"),
			version.WithDefault("v1"),
		))

		// Register a versioned route
		v1 := r.Version("v1")
		v1.GET("/test", func(c *Context) {
			c.String(http.StatusOK, "OK")
		})

		// The fact that this doesn't panic means alignment is correct
		t.Log("Atomic operations on versionTrees completed successfully")
	})
}

// BenchmarkAlignmentImpact measures if proper alignment provides any
// measurable performance benefit.
func BenchmarkAlignmentImpact(b *testing.B) {
	r := MustNew()
	r.GET("/users/:id", func(_ *Context) {})

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		// This accesses atomic fields, testing if alignment impacts performance
		_ = r.getTreeForMethodDirect(http.MethodGet)
	}
}
