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

package benchmarks

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/beego/beego/v2/server/web"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/gin-gonic/gin"
	"github.com/go-chi/chi/v5"
	"github.com/gofiber/fiber/v2"
	"github.com/labstack/echo/v4"

	beecontext "github.com/beego/beego/v2/server/web/context"
	fiberadaptor "github.com/gofiber/fiber/v2/middleware/adaptor"
	fiberv3 "github.com/gofiber/fiber/v3"
	fiberadaptorv3 "github.com/gofiber/fiber/v3/middleware/adaptor"
	router "rivaas.dev/router"
)

// Router Comparison Benchmarks
//
// This file contains comparative benchmarks between rivaas/router and other
// popular Go web frameworks. These benchmarks are isolated in a separate
// module to avoid polluting the main module's dependencies.
//
// All frameworks use the same route structure and response pattern (direct
// writes via io.WriteString / WriteString, no string concatenation or
// fmt.Sprintf) to minimize handler overhead and isolate router dispatch cost.
//
// To run these benchmarks:
//
//	cd benchmarks
//	go test -bench=.

// setupRivaas returns an http.Handler for the Rivaas router with all three routes registered.
// No Warmup() is called for fair comparison with other frameworks.
// Handlers use io.WriteString(c.Response, ...) to avoid string concatenation allocations.
func setupRivaas() http.Handler {
	r := router.MustNew()
	r.GET("/", func(c *router.Context) {
		io.WriteString(c.Response, "Hello") //nolint:errcheck // ignored in benchmark
	})
	r.GET("/users/:id", func(c *router.Context) {
		io.WriteString(c.Response, "User: ")      //nolint:errcheck // ignored in benchmark
		io.WriteString(c.Response, c.Param("id")) //nolint:errcheck // ignored in benchmark
	})
	r.GET("/users/:id/posts/:post_id", func(c *router.Context) {
		io.WriteString(c.Response, "User: ")           //nolint:errcheck // ignored in benchmark
		io.WriteString(c.Response, c.Param("id"))      //nolint:errcheck // ignored in benchmark
		io.WriteString(c.Response, ", Post: ")         //nolint:errcheck // ignored in benchmark
		io.WriteString(c.Response, c.Param("post_id")) //nolint:errcheck // ignored in benchmark
	})
	return r
}

// setupStdMux returns an http.Handler for net/http ServeMux with Go 1.22+ dynamic routing.
func setupStdMux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, "Hello") //nolint:errcheck // ignored in benchmark
	})
	mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "User: ")          //nolint:errcheck // ignored in benchmark
		io.WriteString(w, r.PathValue("id")) //nolint:errcheck // ignored in benchmark
	})
	mux.HandleFunc("GET /users/{id}/posts/{post_id}", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "User: ")               //nolint:errcheck // ignored in benchmark
		io.WriteString(w, r.PathValue("id"))      //nolint:errcheck // ignored in benchmark
		io.WriteString(w, ", Post: ")             //nolint:errcheck // ignored in benchmark
		io.WriteString(w, r.PathValue("post_id")) //nolint:errcheck // ignored in benchmark
	})
	return mux
}

// setupGin returns an http.Handler for Gin in ReleaseMode. Uses io.WriteString(c.Writer, ...)
// for direct writes to avoid allocations and match other frameworks.
func setupGin() http.Handler {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.GET("/", func(c *gin.Context) {
		io.WriteString(c.Writer, "Hello") //nolint:errcheck // ignored in benchmark
	})
	r.GET("/users/:id", func(c *gin.Context) {
		io.WriteString(c.Writer, "User: ")      //nolint:errcheck // ignored in benchmark
		io.WriteString(c.Writer, c.Param("id")) //nolint:errcheck // ignored in benchmark
	})
	r.GET("/users/:id/posts/:post_id", func(c *gin.Context) {
		io.WriteString(c.Writer, "User: ")           //nolint:errcheck // ignored in benchmark
		io.WriteString(c.Writer, c.Param("id"))      //nolint:errcheck // ignored in benchmark
		io.WriteString(c.Writer, ", Post: ")         //nolint:errcheck // ignored in benchmark
		io.WriteString(c.Writer, c.Param("post_id")) //nolint:errcheck // ignored in benchmark
	})
	return r
}

// setupEcho returns an http.Handler for Echo.
func setupEcho() http.Handler {
	e := echo.New()
	e.GET("/", func(c echo.Context) error {
		io.WriteString(c.Response(), "Hello") //nolint:errcheck // ignored in benchmark
		return nil
	})
	e.GET("/users/:id", func(c echo.Context) error {
		io.WriteString(c.Response(), "User: ")      //nolint:errcheck // ignored in benchmark
		io.WriteString(c.Response(), c.Param("id")) //nolint:errcheck // ignored in benchmark
		return nil
	})
	e.GET("/users/:id/posts/:post_id", func(c echo.Context) error {
		io.WriteString(c.Response(), "User: ")           //nolint:errcheck // ignored in benchmark
		io.WriteString(c.Response(), c.Param("id"))      //nolint:errcheck // ignored in benchmark
		io.WriteString(c.Response(), ", Post: ")         //nolint:errcheck // ignored in benchmark
		io.WriteString(c.Response(), c.Param("post_id")) //nolint:errcheck // ignored in benchmark
		return nil
	})
	return e
}

// setupChi returns an http.Handler for Chi.
func setupChi() http.Handler {
	r := chi.NewRouter()
	r.Get("/", func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, "Hello") //nolint:errcheck // ignored in benchmark
	})
	r.Get("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "User: ")              //nolint:errcheck // ignored in benchmark
		io.WriteString(w, chi.URLParam(r, "id")) //nolint:errcheck // ignored in benchmark
	})
	r.Get("/users/{id}/posts/{post_id}", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "User: ")                   //nolint:errcheck // ignored in benchmark
		io.WriteString(w, chi.URLParam(r, "id"))      //nolint:errcheck // ignored in benchmark
		io.WriteString(w, ", Post: ")                 //nolint:errcheck // ignored in benchmark
		io.WriteString(w, chi.URLParam(r, "post_id")) //nolint:errcheck // ignored in benchmark
	})
	return r
}

// setupFiber returns an http.Handler for Fiber via the net/http adaptor.
// The adaptor adds overhead; Fiber is measured this way for httptest compatibility.
func setupFiber() http.Handler {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})
	app.Get("/", func(c *fiber.Ctx) error {
		c.WriteString("Hello") //nolint:errcheck // ignored in benchmark
		return nil
	})
	app.Get("/users/:id", func(c *fiber.Ctx) error {
		c.WriteString("User: ")       //nolint:errcheck // ignored in benchmark
		c.WriteString(c.Params("id")) //nolint:errcheck // ignored in benchmark
		return nil
	})
	app.Get("/users/:id/posts/:post_id", func(c *fiber.Ctx) error {
		c.WriteString("User: ")            //nolint:errcheck // ignored in benchmark
		c.WriteString(c.Params("id"))      //nolint:errcheck // ignored in benchmark
		c.WriteString(", Post: ")          //nolint:errcheck // ignored in benchmark
		c.WriteString(c.Params("post_id")) //nolint:errcheck // ignored in benchmark
		return nil
	})
	return fiberadaptor.FiberApp(app)
}

// setupFiberV3 returns an http.Handler for Fiber v3 via the net/http adaptor.
// Same measurement approach as Fiber v2 for comparable results.
func setupFiberV3() http.Handler {
	app := fiberv3.New()
	app.Get("/", func(c fiberv3.Ctx) error {
		c.Write([]byte("Hello")) //nolint:errcheck // ignored in benchmark
		return nil
	})
	app.Get("/users/:id", func(c fiberv3.Ctx) error {
		c.Write([]byte("User: "))             //nolint:errcheck // ignored in benchmark
		c.Write([]byte(c.Req().Params("id"))) //nolint:errcheck // ignored in benchmark
		return nil
	})
	app.Get("/users/:id/posts/:post_id", func(c fiberv3.Ctx) error {
		c.Write([]byte("User: "))                  //nolint:errcheck // ignored in benchmark
		c.Write([]byte(c.Req().Params("id")))      //nolint:errcheck // ignored in benchmark
		c.Write([]byte(", Post: "))                //nolint:errcheck // ignored in benchmark
		c.Write([]byte(c.Req().Params("post_id"))) //nolint:errcheck // ignored in benchmark
		return nil
	})
	return fiberadaptorv3.FiberApp(app)
}

// setupHertz returns a Hertz engine with the same three routes. Hertz does not expose http.Handler;
// benchmarks use ut.PerformRequest (Hertz's native test API) for the same request flow.
func setupHertz() *server.Hertz {
	h := server.New(server.WithDisablePrintRoute(true))
	h.GET("/", func(_ context.Context, c *app.RequestContext) {
		c.WriteString("Hello") //nolint:errcheck // ignored in benchmark
	})
	h.GET("/users/:id", func(_ context.Context, c *app.RequestContext) {
		c.WriteString("User: ")      //nolint:errcheck // ignored in benchmark
		c.WriteString(c.Param("id")) //nolint:errcheck // ignored in benchmark
	})
	h.GET("/users/:id/posts/:post_id", func(_ context.Context, c *app.RequestContext) {
		c.WriteString("User: ")           //nolint:errcheck // ignored in benchmark
		c.WriteString(c.Param("id"))      //nolint:errcheck // ignored in benchmark
		c.WriteString(", Post: ")         //nolint:errcheck // ignored in benchmark
		c.WriteString(c.Param("post_id")) //nolint:errcheck // ignored in benchmark
	})
	return h
}

// setupBeego returns an http.Handler for Beego v2 using ControllerRegister with the same three routes.
func setupBeego() http.Handler {
	cr := web.NewControllerRegister()
	cr.Get("/", func(ctx *beecontext.Context) {
		io.WriteString(ctx.ResponseWriter, "Hello") //nolint:errcheck // ignored in benchmark
	})
	cr.Get("/users/:id", func(ctx *beecontext.Context) {
		io.WriteString(ctx.ResponseWriter, "User: ")               //nolint:errcheck // ignored in benchmark
		io.WriteString(ctx.ResponseWriter, ctx.Input.Param(":id")) //nolint:errcheck // ignored in benchmark
	})
	cr.Get("/users/:id/posts/:post_id", func(ctx *beecontext.Context) {
		io.WriteString(ctx.ResponseWriter, "User: ")                    //nolint:errcheck // ignored in benchmark
		io.WriteString(ctx.ResponseWriter, ctx.Input.Param(":id"))      //nolint:errcheck // ignored in benchmark
		io.WriteString(ctx.ResponseWriter, ", Post: ")                  //nolint:errcheck // ignored in benchmark
		io.WriteString(ctx.ResponseWriter, ctx.Input.Param(":post_id")) //nolint:errcheck // ignored in benchmark
	})
	return cr
}

// runBenchHertz runs the benchmark loop for Hertz using ut.PerformRequest (no http.Handler).
func runBenchHertz(b *testing.B, h *server.Hertz, method, path string) {
	b.ResetTimer()
	for b.Loop() {
		rec := ut.PerformRequest(h.Engine, method, path, nil)
		_ = rec
	}
}

// runBench runs the benchmark loop: reset recorder, call ServeHTTP. Shared by all framework benchmarks.
func runBench(b *testing.B, h http.Handler, w *httptest.ResponseRecorder, req *http.Request) {
	b.ResetTimer()
	for b.Loop() {
		w.Body.Reset()
		w.Code = 0
		w.Flushed = false
		h.ServeHTTP(w, req)
	}
}

func BenchmarkStatic(b *testing.B) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	b.Run("Rivaas", func(b *testing.B) {
		runBench(b, setupRivaas(), w, req)
	})
	b.Run("StdMux", func(b *testing.B) {
		runBench(b, setupStdMux(), w, req)
	})
	b.Run("Gin", func(b *testing.B) {
		runBench(b, setupGin(), w, req)
	})
	b.Run("Echo", func(b *testing.B) {
		runBench(b, setupEcho(), w, req)
	})
	b.Run("Chi", func(b *testing.B) {
		runBench(b, setupChi(), w, req)
	})
	b.Run("Fiber", func(b *testing.B) {
		runBench(b, setupFiber(), w, req)
	})
	b.Run("FiberV3", func(b *testing.B) {
		runBench(b, setupFiberV3(), w, req)
	})
	b.Run("Hertz", func(b *testing.B) {
		runBenchHertz(b, setupHertz(), http.MethodGet, "/")
	})
	b.Run("Beego", func(b *testing.B) {
		runBench(b, setupBeego(), w, req)
	})
}

func BenchmarkOneParam(b *testing.B) {
	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	w := httptest.NewRecorder()

	b.Run("Rivaas", func(b *testing.B) {
		runBench(b, setupRivaas(), w, req)
	})
	b.Run("StdMux", func(b *testing.B) {
		runBench(b, setupStdMux(), w, req)
	})
	b.Run("Gin", func(b *testing.B) {
		runBench(b, setupGin(), w, req)
	})
	b.Run("Echo", func(b *testing.B) {
		runBench(b, setupEcho(), w, req)
	})
	b.Run("Chi", func(b *testing.B) {
		runBench(b, setupChi(), w, req)
	})
	b.Run("Fiber", func(b *testing.B) {
		runBench(b, setupFiber(), w, req)
	})
	b.Run("FiberV3", func(b *testing.B) {
		runBench(b, setupFiberV3(), w, req)
	})
	b.Run("Hertz", func(b *testing.B) {
		runBenchHertz(b, setupHertz(), http.MethodGet, "/users/123")
	})
	b.Run("Beego", func(b *testing.B) {
		runBench(b, setupBeego(), w, req)
	})
}

func BenchmarkTwoParams(b *testing.B) {
	req := httptest.NewRequest(http.MethodGet, "/users/123/posts/456", nil)
	w := httptest.NewRecorder()

	b.Run("Rivaas", func(b *testing.B) {
		runBench(b, setupRivaas(), w, req)
	})
	b.Run("StdMux", func(b *testing.B) {
		runBench(b, setupStdMux(), w, req)
	})
	b.Run("Gin", func(b *testing.B) {
		runBench(b, setupGin(), w, req)
	})
	b.Run("Echo", func(b *testing.B) {
		runBench(b, setupEcho(), w, req)
	})
	b.Run("Chi", func(b *testing.B) {
		runBench(b, setupChi(), w, req)
	})
	b.Run("Fiber", func(b *testing.B) {
		runBench(b, setupFiber(), w, req)
	})
	b.Run("FiberV3", func(b *testing.B) {
		runBench(b, setupFiberV3(), w, req)
	})
	b.Run("Hertz", func(b *testing.B) {
		runBenchHertz(b, setupHertz(), http.MethodGet, "/users/123/posts/456")
	})
	b.Run("Beego", func(b *testing.B) {
		runBench(b, setupBeego(), w, req)
	})
}
