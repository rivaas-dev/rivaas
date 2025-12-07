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
	"net/http/httptest"
	"testing"
)

// BenchmarkContextPooling tests context pooling with allocation tracking
func BenchmarkContextPooling(b *testing.B) {
	r := MustNew()
	r.GET("/users/:id", func(c *Context) {
		c.Status(200)
	})

	req := httptest.NewRequest("GET", "/users/123", nil)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

// BenchmarkContextPooling_StaticRoute tests static route with allocation tracking
func BenchmarkContextPooling_StaticRoute(b *testing.B) {
	r := MustNew()
	r.GET("/health", func(c *Context) {
		c.Status(200)
	})

	req := httptest.NewRequest("GET", "/health", nil)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

// BenchmarkContextPooling_MultiParam tests multiple parameters with allocation tracking
func BenchmarkContextPooling_MultiParam(b *testing.B) {
	r := MustNew()
	r.GET("/users/:uid/posts/:pid/comments/:cid", func(c *Context) {
		c.Status(200)
	})

	req := httptest.NewRequest("GET", "/users/123/posts/456/comments/789", nil)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

// BenchmarkParamLookup tests parameter lookup to verify stack allocation
func BenchmarkParamLookup(b *testing.B) {
	ctx := &Context{}
	ctx.paramKeys[0] = "id"
	ctx.paramValues[0] = "123"
	ctx.paramCount = 1

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_ = ctx.Param("id") // Should not allocate
	}
}

// BenchmarkParamLookup_Multiple tests multiple param lookups
func BenchmarkParamLookup_Multiple(b *testing.B) {
	ctx := &Context{}
	ctx.paramKeys[0] = "uid"
	ctx.paramValues[0] = "123"
	ctx.paramKeys[1] = "pid"
	ctx.paramValues[1] = "456"
	ctx.paramKeys[2] = "cid"
	ctx.paramValues[2] = "789"
	ctx.paramCount = 3

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_ = ctx.Param("uid")
		_ = ctx.Param("pid")
		_ = ctx.Param("cid")
	}
}

// BenchmarkParamLookup_Fallback tests fallback to map (>8 params)
func BenchmarkParamLookup_Fallback(b *testing.B) {
	ctx := &Context{}
	// Fill array completely
	for i := range 8 {
		ctx.paramKeys[i] = "key" + string(rune('0'+i))
		ctx.paramValues[i] = "val" + string(rune('0'+i))
	}
	ctx.paramCount = 8

	// Add extra param to map
	ctx.Params = make(map[string]string)
	ctx.Params["p9"] = "v9"

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_ = ctx.Param("p9") // Should use map
	}
}

// BenchmarkResponseWriter_Status tests status writing
func BenchmarkResponseWriter_Status(b *testing.B) {
	r := MustNew()
	r.GET("/test", func(c *Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

// BenchmarkResponseWriter_String tests string response
func BenchmarkResponseWriter_String(b *testing.B) {
	r := MustNew()
	r.GET("/test", func(c *Context) {
		c.String(http.StatusOK, "Hello, World!")
	})

	req := httptest.NewRequest("GET", "/test", nil)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

// BenchmarkResponseWriter_JSON tests JSON response allocations
func BenchmarkResponseWriter_JSON(b *testing.B) {
	r := MustNew()
	r.GET("/test", func(c *Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "hello"})
	})

	req := httptest.NewRequest("GET", "/test", nil)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

// BenchmarkMiddlewareChain tests middleware chain with allocations
func BenchmarkMiddlewareChain(b *testing.B) {
	r := MustNew()

	// Add 3 middleware
	r.Use(func(c *Context) {
		c.Next()
	})
	r.Use(func(c *Context) {
		c.Next()
	})
	r.Use(func(c *Context) {
		c.Next()
	})

	r.GET("/test", func(c *Context) {
		c.Status(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

// BenchmarkContextReset tests context reset allocations
func BenchmarkContextReset(b *testing.B) {
	ctx := &Context{}
	ctx.paramKeys[0] = "id"
	ctx.paramValues[0] = "123"
	ctx.paramKeys[1] = "name"
	ctx.paramValues[1] = "john"
	ctx.paramCount = 2
	ctx.Params = make(map[string]string)
	ctx.Params["extra"] = "value"

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		ctx.reset()
	}
}

// BenchmarkHandlerExecution tests pure handler execution overhead
func BenchmarkHandlerExecution(b *testing.B) {
	ctx := &Context{
		Response: httptest.NewRecorder(),
	}

	handler := func(c *Context) {
		c.Status(200)
	}

	ctx.handlers = []HandlerFunc{handler}
	ctx.index = -1

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		ctx.index = -1
		ctx.Next()
	}
}
