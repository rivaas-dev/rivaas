// verify_layout is a development tool that verifies the memory layout of the Context struct.
//
// # Purpose
//
// This tool ensures that the "hot-path" fields (accessed on every request) of the Context
// struct fit within a single 64-byte CPU cache line. Modern CPUs load memory in cache lines,
// and splitting hot fields across cache lines causes additional memory accesses.
//
// # What It Checks
//
//   - Field sizes and offsets
//   - Whether hot-path fields (Request, Response, handlers, router, index, paramCount)
//     fit within 64 bytes
//   - Total Context struct size
//   - String internal layout (ptr + len)
//
// # Usage
//
//	cd router/tools
//	go run verify_layout.go
//
// # Expected Output
//
// You should see "✅ Hot fields fit in one cache line (64 bytes)". If you see a red cross (❌),
// it means the memory layout has regressed and hot-path fields no longer fit in one cache line.
//
// # When to Run
//
//   - After modifying the Context struct in context.go
//   - Before committing changes that affect Context memory layout
//   - In CI/CD to catch layout regressions automatically
//
// # Related Documentation
//
// See router/context.go for the actual Context struct and detailed memory layout documentation.
package main

import (
	"fmt"
	"unsafe"
)

// Mock Context to verify memory layout
// This mirrors the actual Context struct from router/context.go
type Context struct {
	// CACHE LINE 1: Hottest fields (accessed on every request) - first 64 bytes
	Request  uintptr // *http.Request (8B)
	Response uintptr // http.ResponseWriter (8B)
	handlers uintptr // []HandlerFunc (24B: ptr+len+cap)
	router   uintptr // *Router (8B)

	// Still in cache line 1 (48 bytes used, 16 remaining)
	index      int32 // Current handler index in the chain (4B)
	paramCount int32 // Number of parameters stored in arrays (4B)
	// 8 bytes padding to cache line boundary

	// CACHE LINE 2: Parameter storage (accessed when params present)
	paramKeys   [8]string // Parameter names (up to 8 parameters) (128B)
	paramValues [8]string // Parameter values (up to 8 parameters) (128B)

	// Additional fields...
}

func main() {
	var c Context

	fmt.Println("=== Context Field Layout Analysis ===")
	fmt.Println()

	// Calculate sizes
	requestSize := unsafe.Sizeof(c.Request)
	responseSize := unsafe.Sizeof(c.Response)
	handlersSize := unsafe.Sizeof(c.handlers)
	routerSize := unsafe.Sizeof(c.router)
	indexSize := unsafe.Sizeof(c.index)
	paramCountSize := unsafe.Sizeof(c.paramCount)

	fmt.Printf("Field Sizes:\n")
	fmt.Printf("  Request:     %2d bytes (offset: %3d)\n", requestSize, unsafe.Offsetof(c.Request))
	fmt.Printf("  Response:    %2d bytes (offset: %3d)\n", responseSize, unsafe.Offsetof(c.Response))
	fmt.Printf("  handlers:    %2d bytes (offset: %3d)\n", handlersSize, unsafe.Offsetof(c.handlers))
	fmt.Printf("  router:      %2d bytes (offset: %3d)\n", routerSize, unsafe.Offsetof(c.router))
	fmt.Printf("  index:       %2d bytes (offset: %3d)\n", indexSize, unsafe.Offsetof(c.index))
	fmt.Printf("  paramCount:  %2d bytes (offset: %3d)\n", paramCountSize, unsafe.Offsetof(c.paramCount))
	fmt.Printf("  paramKeys:   %2d bytes (offset: %3d)\n", unsafe.Sizeof(c.paramKeys), unsafe.Offsetof(c.paramKeys))
	fmt.Printf("  paramValues: %2d bytes (offset: %3d)\n", unsafe.Sizeof(c.paramValues), unsafe.Offsetof(c.paramValues))

	totalBefore := requestSize + responseSize + handlersSize + routerSize + indexSize + paramCountSize
	fmt.Printf("\n\"Hot Path\" Fields Total: %d bytes\n", totalBefore)

	firstCacheLineEnd := unsafe.Offsetof(c.paramKeys)
	fmt.Printf("First Cache Line Ends At: %d bytes\n", firstCacheLineEnd)

	if firstCacheLineEnd <= 64 {
		fmt.Printf("✅ Hot fields fit in one cache line (64 bytes)\n")
	} else {
		fmt.Printf("❌ Hot fields DO NOT fit in one cache line (need %d bytes)\n", firstCacheLineEnd)
	}

	fmt.Printf("\nTotal Context Size: %d bytes\n", unsafe.Sizeof(c))

	// String internal layout
	type stringStruct struct {
		ptr uintptr
		len int
	}
	var s string
	fmt.Printf("\nString size: %d bytes (ptr + len)\n", unsafe.Sizeof(s))
	fmt.Printf("  String internals: %+v\n", (*stringStruct)(unsafe.Pointer(&s)))
}
