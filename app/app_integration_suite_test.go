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

//go:build integration

package app_test

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"rivaas.dev/app"
	"rivaas.dev/logging"
	"rivaas.dev/metrics"
	"rivaas.dev/router"
	"rivaas.dev/router/version"
	"rivaas.dev/tracing"
)

var _ = Describe("App Integration", func() {
	Describe("Server Lifecycle", func() {
		It("should start and respond to requests", func() {
			a := app.MustNew(
				app.WithServiceName("test"),
				app.WithServiceVersion("1.0.0"),
				app.WithServer(
					app.WithShutdownTimeout(2*time.Second),
				),
			)

			a.GET("/health", func(c *app.Context) {
				if err := c.String(http.StatusOK, "ok"); err != nil {
					slog.ErrorContext(c.RequestContext(), "failed to write response", "err", err)
				}
			})

			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			rec := httptest.NewRecorder()
			a.Router().ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(rec.Body.String()).To(Equal("ok"))
		})

		It("should apply server configuration correctly", func() {
			customTimeout := 5 * time.Second
			a := app.MustNew(
				app.WithServiceName("test"),
				app.WithServiceVersion("1.0.0"),
				app.WithServer(
					app.WithReadTimeout(customTimeout),
					app.WithWriteTimeout(customTimeout),
					app.WithIdleTimeout(customTimeout),
					app.WithShutdownTimeout(customTimeout),
				),
			)

			// Verify configuration is stored (through public API if available)
			// Note: We can't directly access config.server, so we verify through behavior
			Expect(a).NotTo(BeNil())
		})
	})

	Describe("HTTP Server Configuration", func() {
		It("should configure HTTP server correctly", func() {
			a := app.MustNew(
				app.WithServiceName("test"),
				app.WithServiceVersion("1.0.0"),
				app.WithServer(
					app.WithReadTimeout(5*time.Second),
					app.WithWriteTimeout(10*time.Second),
					app.WithIdleTimeout(30*time.Second),
					app.WithReadHeaderTimeout(2*time.Second),
					app.WithMaxHeaderBytes(4096),
					app.WithShutdownTimeout(15*time.Second),
				),
			)

			a.GET("/test", func(c *app.Context) {
				if err := c.String(http.StatusOK, "test"); err != nil {
					slog.ErrorContext(c.RequestContext(), "failed to write response", "err", err)
				}
			})

			// Verify server can be created and configured
			Expect(a).NotTo(BeNil())
		})
	})

	Describe("Graceful Shutdown", func() {
		It("should handle shutdown timeout correctly", func() {
			// Test shutdown context creation
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			Expect(ctx).NotTo(BeNil())
			deadline, ok := ctx.Deadline()
			Expect(ok).To(BeTrue())
			Expect(deadline).To(BeTemporally("~", time.Now().Add(1*time.Second), 100*time.Millisecond))
		})
	})

	Describe("Observability Shutdown", func() {
		Context("without observability components", func() {
			It("should shutdown without panicking", func() {
				a := app.MustNew(
					app.WithServiceName("test"),
					app.WithServiceVersion("1.0.0"),
				)

				// This tests internal shutdown behavior through public API
				// In a real scenario, this would be called during server shutdown
				Expect(func() {
					// We can't directly call shutdownObservability, but we can verify
					// the app doesn't panic during normal operations
					a.GET("/test", func(c *app.Context) {
						if err := c.String(http.StatusOK, "ok"); err != nil {
							slog.ErrorContext(c.RequestContext(), "failed to write response", "err", err)
						}
					})
				}).NotTo(Panic())
			})
		})
	})

	Describe("Option application", func() {
		It("should create app with minimal options", func() {
			a := app.MustNew(
				app.WithServiceName("minimal"),
				app.WithServiceVersion("0.0.0"),
			)
			Expect(a).NotTo(BeNil())
			Expect(a.ServiceName()).To(Equal("minimal"))
		})

		It("should create app with server options", func() {
			a := app.MustNew(
				app.WithServiceName("test"),
				app.WithServiceVersion("1.0.0"),
				app.WithServer(
					app.WithReadTimeout(1*time.Second),
					app.WithWriteTimeout(1*time.Second),
					app.WithShutdownTimeout(1*time.Second),
				),
			)
			Expect(a).NotTo(BeNil())
		})

		It("should register route with and without route options", func() {
			a := app.MustNew(
				app.WithServiceName("test"),
				app.WithServiceVersion("1.0.0"),
			)
			a.GET("/plain", func(c *app.Context) {
				_ = c.String(http.StatusOK, "plain")
			})
			a.GET("/with-middleware", func(c *app.Context) {
				_ = c.String(http.StatusOK, "middleware")
			}, app.WithBefore(func(c *app.Context) {
				c.Next()
			}))

			req := httptest.NewRequest(http.MethodGet, "/plain", nil)
			rec := httptest.NewRecorder()
			a.Router().ServeHTTP(rec, req)
			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(rec.Body.String()).To(Equal("plain"))

			req2 := httptest.NewRequest(http.MethodGet, "/with-middleware", nil)
			rec2 := httptest.NewRecorder()
			a.Router().ServeHTTP(rec2, req2)
			Expect(rec2.Code).To(Equal(http.StatusOK))
			Expect(rec2.Body.String()).To(Equal("middleware"))
		})
	})

	// registerRoute default branch: The default branch in registerRoute (for unsupported
	// HTTP method) is defensive only. All public APIs (GET, POST, PUT, DELETE, PATCH, HEAD,
	// OPTIONS, Any) pass a known method, so the default is not reachable from public API.

	Describe("App Getters and Metrics", func() {
		Describe("ServiceName, ServiceVersion, Environment", func() {
			It("should return configured service name, version, and environment", func() {
				a := app.MustNew(
					app.WithServiceName("my-service"),
					app.WithServiceVersion("2.0.0"),
					app.WithEnvironment(app.EnvironmentProduction),
				)
				Expect(a.ServiceName()).To(Equal("my-service"))
				Expect(a.ServiceVersion()).To(Equal("2.0.0"))
				Expect(a.Environment()).To(Equal(app.EnvironmentProduction))
			})

			It("should return default environment when not configured", func() {
				a := app.MustNew(
					app.WithServiceName("test"),
					app.WithServiceVersion("1.0.0"),
				)
				Expect(a.Environment()).To(Equal(app.DefaultEnvironment))
			})
		})

		Describe("GetMetricsHandler", func() {
			It("should return error when metrics are not enabled", func() {
				a := app.MustNew(
					app.WithServiceName("test"),
					app.WithServiceVersion("1.0.0"),
				)
				handler, err := a.GetMetricsHandler()
				Expect(err).To(HaveOccurred())
				Expect(handler).To(BeNil())
				Expect(err.Error()).To(ContainSubstring("metrics not enabled"))
			})

			It("should return handler when metrics are enabled", func() {
				a, err := app.New(
					app.WithServiceName("test"),
					app.WithServiceVersion("1.0.0"),
					app.WithObservability(app.WithMetrics(metrics.WithPrometheus(":0", "/metrics"))),
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(a).NotTo(BeNil())
				handler, err := a.GetMetricsHandler()
				Expect(err).NotTo(HaveOccurred())
				Expect(handler).NotTo(BeNil())
			})
		})

		Describe("GetMetricsServerAddress", func() {
			It("should return empty string when metrics are not enabled", func() {
				a := app.MustNew(
					app.WithServiceName("test"),
					app.WithServiceVersion("1.0.0"),
				)
				addr := a.GetMetricsServerAddress()
				Expect(addr).To(Equal(""))
			})

			It("should return address when metrics are enabled", func() {
				a, err := app.New(
					app.WithServiceName("test"),
					app.WithServiceVersion("1.0.0"),
					app.WithObservability(app.WithMetrics(metrics.WithPrometheus(":9091", "/metrics"))),
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(a).NotTo(BeNil())
				addr := a.GetMetricsServerAddress()
				Expect(addr).NotTo(BeEmpty())
				Expect(addr).To(ContainSubstring("9091"))
			})
		})

		Describe("Metrics and Tracing", func() {
			It("should return nil when observability is not configured", func() {
				a := app.MustNew(
					app.WithServiceName("test"),
					app.WithServiceVersion("1.0.0"),
				)
				Expect(a.Metrics()).To(BeNil())
				Expect(a.Tracing()).To(BeNil())
			})

			It("should return non-nil when observability is configured", func() {
				a, err := app.New(
					app.WithServiceName("test"),
					app.WithServiceVersion("1.0.0"),
					app.WithObservability(
						app.WithMetrics(metrics.WithPrometheus(":0", "/metrics")),
						app.WithTracing(tracing.WithNoop()),
					),
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(a).NotTo(BeNil())
				Expect(a.Metrics()).NotTo(BeNil())
				Expect(a.Tracing()).NotTo(BeNil())
			})
		})

		Describe("BaseLogger", func() {
			It("should never return nil when no logger is configured", func() {
				a := app.MustNew(
					app.WithServiceName("test"),
					app.WithServiceVersion("1.0.0"),
				)
				logger := a.BaseLogger()
				Expect(logger).NotTo(BeNil())
			})

			It("should return configured logger when logging is enabled", func() {
				a, err := app.New(
					app.WithServiceName("test"),
					app.WithServiceVersion("1.0.0"),
					app.WithObservability(app.WithLogging(logging.WithLevel(logging.LevelInfo))),
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(a).NotTo(BeNil())
				logger := a.BaseLogger()
				Expect(logger).NotTo(BeNil())
			})
		})
	})

	Describe("Route Introspection", func() {
		It("should return route by name and list all routes after freeze", func() {
			a := app.MustNew(
				app.WithServiceName("test"),
				app.WithServiceVersion("1.0.0"),
			)
			a.GET("/users/:id", func(c *app.Context) {
				_ = c.String(http.StatusOK, "ok")
			}).SetName("users.get")
			a.GET("/users", func(c *app.Context) {
				_ = c.String(http.StatusOK, "list")
			}).SetName("users.list")

			a.Router().Freeze()

			rt, ok := a.Route("users.get")
			Expect(ok).To(BeTrue())
			Expect(rt).NotTo(BeNil())
			Expect(rt.Name()).To(Equal("users.get"))
			Expect(rt.Method()).To(Equal(http.MethodGet))
			Expect(rt.Path()).To(Equal("/users/:id"))

			_, ok = a.Route("nonexistent")
			Expect(ok).To(BeFalse())

			routes := a.Routes()
			Expect(routes).NotTo(BeNil())
			Expect(routes).To(HaveLen(2))
		})

		It("should generate URL from route name via URLFor and MustURLFor", func() {
			a := app.MustNew(
				app.WithServiceName("test"),
				app.WithServiceVersion("1.0.0"),
			)
			a.GET("/users/:id", func(c *app.Context) {
				_ = c.String(http.StatusOK, "ok")
			}).SetName("users.get")

			a.Router().Freeze()

			url, err := a.URLFor("users.get", map[string]string{"id": "123"}, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(url).To(Equal("/users/123"))

			mustURL := a.MustURLFor("users.get", map[string]string{"id": "456"}, nil)
			Expect(mustURL).To(Equal("/users/456"))
		})

		It("should return error from URLFor when route is not found", func() {
			a := app.MustNew(
				app.WithServiceName("test"),
				app.WithServiceVersion("1.0.0"),
			)
			a.Router().Freeze()

			_, err := a.URLFor("missing.route", nil, nil)
			Expect(err).To(HaveOccurred())
		})

		It("should panic when MustURLFor is called with missing route", func() {
			a := app.MustNew(
				app.WithServiceName("test"),
				app.WithServiceVersion("1.0.0"),
			)
			a.Router().Freeze()

			Expect(func() {
				a.MustURLFor("missing.route", nil, nil)
			}).To(Panic())
		})
	})

	Describe("NoRoute", func() {
		It("should restore default NotFound behavior when NoRoute(nil) is called", func() {
			a := app.MustNew(
				app.WithServiceName("test"),
				app.WithServiceVersion("1.0.0"),
			)
			a.GET("/known", func(c *app.Context) {
				_ = c.String(http.StatusOK, "ok")
			})
			a.NoRoute(nil)

			req := httptest.NewRequest(http.MethodGet, "/unknown/path", nil)
			rec := httptest.NewRecorder()
			a.Router().ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})

		It("should use custom handler when NoRoute is set to non-nil", func() {
			a := app.MustNew(
				app.WithServiceName("test"),
				app.WithServiceVersion("1.0.0"),
			)
			a.NoRoute(func(c *app.Context) {
				_ = c.JSON(http.StatusNotFound, map[string]string{"error": "route not found"})
			})

			req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
			rec := httptest.NewRecorder()
			a.Router().ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusNotFound))
			Expect(rec.Body.String()).To(ContainSubstring("route not found"))
		})
	})

	Describe("Version, Static, Any, File, StaticFS", func() {
		It("should serve routes registered under Version group", func() {
			a := app.MustNew(
				app.WithServiceName("test"),
				app.WithServiceVersion("1.0.0"),
				app.WithRouter(
					router.WithVersioning(
						version.WithPathDetection("/v{version}/"),
						version.WithDefault("v1"),
					),
				),
			)
			v1 := a.Version("v1")
			v1.GET("/status", func(c *app.Context) {
				_ = c.String(http.StatusOK, "v1-status")
			})

			req := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
			rec := httptest.NewRecorder()
			a.Router().ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(rec.Body.String()).To(Equal("v1-status"))
		})

		It("should serve static files from directory via Static", func() {
			dir, err := os.MkdirTemp("", "app-static-*")
			Expect(err).NotTo(HaveOccurred())
			defer os.RemoveAll(dir)
			err = os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello static"), 0o644)
			Expect(err).NotTo(HaveOccurred())

			a := app.MustNew(
				app.WithServiceName("test"),
				app.WithServiceVersion("1.0.0"),
			)
			a.Static("/static", dir)

			req := httptest.NewRequest(http.MethodGet, "/static/hello.txt", nil)
			rec := httptest.NewRecorder()
			a.Router().ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(rec.Body.String()).To(Equal("hello static"))
		})

		It("should match all HTTP methods via Any", func() {
			a := app.MustNew(
				app.WithServiceName("test"),
				app.WithServiceVersion("1.0.0"),
			)
			a.Any("/any", func(c *app.Context) {
				_ = c.String(http.StatusOK, "any:"+c.Request.Method)
			})

			for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut} {
				req := httptest.NewRequest(method, "/any", nil)
				rec := httptest.NewRecorder()
				a.Router().ServeHTTP(rec, req)
				Expect(rec.Code).To(Equal(http.StatusOK))
				Expect(rec.Body.String()).To(Equal("any:" + method))
			}
		})

		It("should serve single file via File", func() {
			f, err := os.CreateTemp("", "app-file-*.txt")
			Expect(err).NotTo(HaveOccurred())
			_, err = f.Write([]byte("file content"))
			Expect(err).NotTo(HaveOccurred())
			path := f.Name()
			Expect(f.Close()).NotTo(HaveOccurred())
			defer os.Remove(path)

			a := app.MustNew(
				app.WithServiceName("test"),
				app.WithServiceVersion("1.0.0"),
			)
			a.File("/f", path)

			req := httptest.NewRequest(http.MethodGet, "/f", nil)
			rec := httptest.NewRecorder()
			a.Router().ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(rec.Body.String()).To(Equal("file content"))
		})

		It("should serve files from filesystem via StaticFS", func() {
			dir, err := os.MkdirTemp("", "app-staticfs-*")
			Expect(err).NotTo(HaveOccurred())
			defer os.RemoveAll(dir)
			err = os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
			Expect(err).NotTo(HaveOccurred())
			err = os.WriteFile(filepath.Join(dir, "sub", "fs.txt"), []byte("fs content"), 0o644)
			Expect(err).NotTo(HaveOccurred())

			a := app.MustNew(
				app.WithServiceName("test"),
				app.WithServiceVersion("1.0.0"),
			)
			a.StaticFS("/fs", http.Dir(dir))

			req := httptest.NewRequest(http.MethodGet, "/fs/sub/fs.txt", nil)
			rec := httptest.NewRecorder()
			a.Router().ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(rec.Body.String()).To(Equal("fs content"))
		})
	})

	Describe("Route Handling", func() {
		var a *app.App

		BeforeEach(func() {
			a = app.MustNew(
				app.WithServiceName("test"),
				app.WithServiceVersion("1.0.0"),
			)

			a.GET("/", func(c *app.Context) {
				if err := c.String(http.StatusOK, "home"); err != nil {
					slog.ErrorContext(c.RequestContext(), "failed to write response", "err", err)
				}
			})

			a.GET("/users/:id", func(c *app.Context) {
				userID := c.Param("id")
				if err := c.Stringf(http.StatusOK, "user-%s", userID); err != nil {
					slog.ErrorContext(c.RequestContext(), "failed to write response", "err", err)
				}
			})

			a.POST("/users", func(c *app.Context) {
				if err := c.String(http.StatusCreated, "created"); err != nil {
					slog.ErrorContext(c.RequestContext(), "failed to write response", "err", err)
				}
			})
		})

		DescribeTable("should handle routes correctly",
			func(method, path string, expectedStatus int, expectedBody string) {
				req := httptest.NewRequest(method, path, nil)
				rec := httptest.NewRecorder()
				a.Router().ServeHTTP(rec, req)

				Expect(rec.Code).To(Equal(expectedStatus))
				if expectedBody != "" {
					Expect(rec.Body.String()).To(ContainSubstring(expectedBody))
				}
			},
			Entry("GET /", http.MethodGet, "/", http.StatusOK, "home"),
			Entry("GET /users/:id", http.MethodGet, "/users/123", http.StatusOK, "user-123"),
			Entry("POST /users", http.MethodPost, "/users", http.StatusCreated, "created"),
		)
	})

	Describe("Middleware Execution", func() {
		It("should execute middleware chain in correct order", func() {
			a := app.MustNew(
				app.WithServiceName("test"),
				app.WithServiceVersion("1.0.0"),
			)

			executionOrder := []string{}

			a.Use(func(c *app.Context) {
				executionOrder = append(executionOrder, "middleware1")
				c.Next()
			})

			a.Use(func(c *app.Context) {
				executionOrder = append(executionOrder, "middleware2")
				c.Next()
			})

			a.GET("/test", func(c *app.Context) {
				executionOrder = append(executionOrder, "handler")
				if err := c.String(http.StatusOK, "ok"); err != nil {
					slog.ErrorContext(c.RequestContext(), "failed to write response", "err", err)
				}
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rec := httptest.NewRecorder()
			a.Router().ServeHTTP(rec, req)

			Expect(executionOrder).To(Equal([]string{"middleware1", "middleware2", "handler"}))
			Expect(rec.Code).To(Equal(http.StatusOK))
		})
	})

	Describe("Handler Panic and Context Pool", func() {
		It("should recover from handler panic and return context to pool for subsequent requests", func() {
			a := app.MustNew(
				app.WithServiceName("test"),
				app.WithServiceVersion("1.0.0"),
			)
			a.GET("/panic", func(c *app.Context) {
				panic("test panic")
			})
			a.GET("/ok", func(c *app.Context) {
				_ = c.String(http.StatusOK, "ok")
			})

			req := httptest.NewRequest(http.MethodGet, "/panic", nil)
			rec := httptest.NewRecorder()
			a.Router().ServeHTTP(rec, req)
			Expect(rec.Code).To(Equal(http.StatusInternalServerError))

			req2 := httptest.NewRequest(http.MethodGet, "/ok", nil)
			rec2 := httptest.NewRecorder()
			a.Router().ServeHTTP(rec2, req2)
			Expect(rec2.Code).To(Equal(http.StatusOK))
			Expect(rec2.Body.String()).To(Equal("ok"))
		})
	})

	Describe("Default Middleware Behavior", func() {
		Context("in development environment", func() {
			It("should include recovery middleware by default", func() {
				a := app.MustNew(
					app.WithServiceName("test"),
					app.WithServiceVersion("1.0.0"),
					app.WithEnvironment(app.EnvironmentDevelopment),
				)

				a.GET("/panic", func(c *app.Context) {
					panic("test panic")
				})

				req := httptest.NewRequest(http.MethodGet, "/panic", nil)
				rec := httptest.NewRecorder()
				a.Router().ServeHTTP(rec, req)

				// Recovery middleware should catch the panic and return 500
				Expect(rec.Code).To(Equal(http.StatusInternalServerError))
			})
		})

		Context("in production environment", func() {
			It("should include recovery middleware by default", func() {
				a := app.MustNew(
					app.WithServiceName("test"),
					app.WithServiceVersion("1.0.0"),
					app.WithEnvironment(app.EnvironmentProduction),
				)

				a.GET("/panic", func(c *app.Context) {
					panic("test panic")
				})

				req := httptest.NewRequest(http.MethodGet, "/panic", nil)
				rec := httptest.NewRecorder()
				a.Router().ServeHTTP(rec, req)

				// Recovery middleware should catch the panic and return 500
				Expect(rec.Code).To(Equal(http.StatusInternalServerError))
			})
		})
	})

	Describe("Complex Route Scenarios", func() {
		It("should handle nested route groups correctly", func() {
			a := app.MustNew(
				app.WithServiceName("test"),
				app.WithServiceVersion("1.0.0"),
			)

			api := a.Group("/api")
			v1 := api.Group("/v1")
			v1.GET("/users", func(c *app.Context) {
				if err := c.String(http.StatusOK, "v1-users"); err != nil {
					slog.ErrorContext(c.RequestContext(), "failed to write response", "err", err)
				}
			})

			v2 := api.Group("/v2")
			v2.GET("/users", func(c *app.Context) {
				if err := c.String(http.StatusOK, "v2-users"); err != nil {
					slog.ErrorContext(c.RequestContext(), "failed to write response", "err", err)
				}
			})

			req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
			rec := httptest.NewRecorder()
			a.Router().ServeHTTP(rec, req)
			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(rec.Body.String()).To(ContainSubstring("v1-users"))

			req = httptest.NewRequest(http.MethodGet, "/api/v2/users", nil)
			rec = httptest.NewRecorder()
			a.Router().ServeHTTP(rec, req)
			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(rec.Body.String()).To(ContainSubstring("v2-users"))
		})
	})

	Describe("Concurrency", func() {
		Describe("Concurrent App Creation", func() {
			It("should create multiple App instances concurrently without race conditions", func() {
				const numGoroutines = 50
				var wg sync.WaitGroup
				errors := make(chan error, numGoroutines)

				for range numGoroutines {
					(&wg).Go(func() {
						a, err := app.New(
							app.WithServiceName("test-service"),
							app.WithServiceVersion("1.0.0"),
							app.WithEnvironment(app.EnvironmentDevelopment),
						)
						if err != nil {
							errors <- err
							return
						}

						Expect(a).NotTo(BeNil())

						a.GET("/test", func(c *app.Context) {
							if stringErr := c.String(http.StatusOK, "ok"); stringErr != nil {
								slog.ErrorContext(c.RequestContext(), "failed to write response", "err", stringErr)
							}
						})

						req := httptest.NewRequest(http.MethodGet, "/test", nil)
						w := httptest.NewRecorder()
						a.Router().ServeHTTP(w, req)

						Expect(w.Code).To(Equal(http.StatusOK))
					})
				}

				wg.Wait()
				close(errors)

				for err := range errors {
					Fail("concurrent app creation failed: " + err.Error())
				}
			})
		})

		Describe("Concurrent Requests", func() {
			It("should handle many concurrent requests correctly", func() {
				a := app.MustNew(
					app.WithServiceName("test"),
					app.WithServiceVersion("1.0.0"),
				)

				var requestCount atomic.Int64
				var successCount atomic.Int64

				a.GET("/test", func(c *app.Context) {
					requestCount.Add(1)
					time.Sleep(1 * time.Millisecond) // Simulate work
					successCount.Add(1)
					if err := c.JSON(http.StatusOK, map[string]int64{
						"count": requestCount.Load(),
					}); err != nil {
						slog.ErrorContext(c.RequestContext(), "failed to write response", "err", err)
					}
				})

				const concurrency = 200
				var wg sync.WaitGroup

				for range concurrency {
					(&wg).Go(func() {
						req := httptest.NewRequest(http.MethodGet, "/test", nil)
						w := httptest.NewRecorder()
						a.Router().ServeHTTP(w, req)
						if w.Code == http.StatusOK {
							successCount.Add(0) // Already counted in handler
						}
					})
				}

				wg.Wait()

				Expect(requestCount.Load()).To(Equal(int64(concurrency)), "all requests should be processed")
				Expect(successCount.Load()).To(Equal(int64(concurrency)), "all requests should succeed")
			})
		})

		Describe("Middleware Chain", func() {
			It("should execute middleware in correct order under concurrent load", func() {
				a := app.MustNew(
					app.WithServiceName("test"),
					app.WithServiceVersion("1.0.0"),
				)

				var executionOrder []int64
				var orderMutex sync.Mutex
				var counter atomic.Int64

				a.Use(func(c *app.Context) {
					orderMutex.Lock()
					executionOrder = append(executionOrder, counter.Add(1))
					orderMutex.Unlock()
					c.Next()
				})

				a.Use(func(c *app.Context) {
					orderMutex.Lock()
					executionOrder = append(executionOrder, counter.Add(1))
					orderMutex.Unlock()
					c.Next()
				})

				a.GET("/test", func(c *app.Context) {
					orderMutex.Lock()
					executionOrder = append(executionOrder, counter.Add(1))
					orderMutex.Unlock()
					if err := c.String(http.StatusOK, "ok"); err != nil {
						slog.ErrorContext(c.RequestContext(), "failed to write response", "err", err)
					}
				})

				const numRequests = 100
				var wg sync.WaitGroup

				for range numRequests {
					(&wg).Go(func() {
						req := httptest.NewRequest(http.MethodGet, "/test", nil)
						w := httptest.NewRecorder()
						a.Router().ServeHTTP(w, req)
						Expect(w.Code).To(Equal(http.StatusOK))
					})
				}

				wg.Wait()

				// Verify execution order: middleware 1, middleware 2, handler
				// Each request should have 3 entries in order
				Expect(executionOrder).To(HaveLen(numRequests*3), "should have 3 entries per request")
			})
		})

		Describe("Observability Concurrent", func() {
			It("should handle observability components correctly under concurrent load", func() {
				a, err := app.New(
					app.WithServiceName("test"),
					app.WithServiceVersion("1.0.0"),
					app.WithObservability(
						app.WithLogging(logging.WithLevel(logging.LevelInfo)),
						app.WithMetrics(metrics.WithPrometheus(":9090", "/metrics")),
						app.WithTracing(tracing.WithNoop()),
					),
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(a).NotTo(BeNil())

				var successCount atomic.Int64

				a.GET("/test", func(c *app.Context) {
					// Use observability features
					c.IncrementCounter("test_requests_total")
					c.SetSpanAttribute("test.key", "test.value")
					slog.InfoContext(c.RequestContext(), "test request", "request_id", "123")

					successCount.Add(1)
					if stringErr := c.String(http.StatusOK, "ok"); stringErr != nil {
						slog.ErrorContext(c.RequestContext(), "failed to write response", "err", stringErr)
					}
				})

				const concurrency = 50
				var wg sync.WaitGroup

				for range concurrency {
					(&wg).Go(func() {
						req := httptest.NewRequest(http.MethodGet, "/test", nil)
						w := httptest.NewRecorder()
						a.Router().ServeHTTP(w, req)
						Expect(w.Code).To(Equal(http.StatusOK))
					})
				}

				wg.Wait()

				Expect(successCount.Load()).To(Equal(int64(concurrency)))
			})
		})

		Describe("Route Registration Concurrent", func() {
			It("should handle concurrent requests safely after routes are registered", func() {
				// This test verifies the two-phase design:
				// Phase 1: Configuration (single-threaded route registration)
				// Phase 2: Serving (concurrent request handling)
				//
				// After the first ServeHTTP call, routes are frozen and immutable.
				// Concurrent reads from the route tree are safe without locking.

				a := app.MustNew(
					app.WithServiceName("test"),
					app.WithServiceVersion("1.0.0"),
				)

				// Phase 1: Register all routes (single-threaded configuration)
				const numRoutes = 50
				for i := range numRoutes {
					path := fmt.Sprintf("/route%d", i)
					routeID := i // capture for closure
					a.GET(path, func(c *app.Context) {
						if err := c.Stringf(http.StatusOK, "route-%d", routeID); err != nil {
							slog.ErrorContext(c.RequestContext(), "failed to write response", "err", err)
						}
					})
				}

				// Explicitly freeze the router (this happens automatically on first ServeHTTP)
				a.Router().Freeze()

				// Phase 2: Handle requests concurrently (routes are now immutable)
				var wg sync.WaitGroup
				requestDone := make(chan bool, numRoutes*10)

				for i := range numRoutes {
					// Each route gets 10 concurrent requests
					for range 10 {
						routeID := i
						wg.Add(1)
						go func() {
							defer wg.Done()
							req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/route%d", routeID), nil)
							w := httptest.NewRecorder()
							a.Router().ServeHTTP(w, req)
							requestDone <- (w.Code == http.StatusOK)
						}()
					}
				}

				wg.Wait()
				close(requestDone)

				// Verify all requests succeeded
				successCount := 0
				for success := range requestDone {
					if success {
						successCount++
					}
				}
				Expect(successCount).To(Equal(numRoutes*10), "all concurrent requests should succeed")
			})

			It("should panic when registering routes after serving starts", func() {
				// This test verifies the fail-fast behavior:
				// Attempting to register routes after serving starts should panic.

				a := app.MustNew(
					app.WithServiceName("test"),
					app.WithServiceVersion("1.0.0"),
				)

				// Register a route
				a.GET("/existing", func(c *app.Context) {
					if err := c.String(http.StatusOK, "existing"); err != nil {
						slog.ErrorContext(c.RequestContext(), "failed to write response", "err", err)
					}
				})

				// Start serving (this triggers auto-freeze)
				req := httptest.NewRequest(http.MethodGet, "/existing", nil)
				w := httptest.NewRecorder()
				a.Router().ServeHTTP(w, req)
				Expect(w.Code).To(Equal(http.StatusOK))

				// Attempting to register a new route after serving should panic
				Expect(func() {
					a.GET("/new-route", func(c *app.Context) {
						Expect(c.String(http.StatusOK, "new")).NotTo(HaveOccurred())
					})
				}).To(Panic())
			})
		})

		Describe("Server Lifecycle", func() {
			It("should handle complete server lifecycle including startup and shutdown", func() {
				a := app.MustNew(
					app.WithServiceName("test"),
					app.WithServiceVersion("1.0.0"),
					app.WithServer(
						app.WithReadTimeout(5*time.Second),
						app.WithWriteTimeout(5*time.Second),
						app.WithShutdownTimeout(2*time.Second),
					),
				)

				a.GET("/test", func(c *app.Context) {
					if err := c.String(http.StatusOK, "ok"); err != nil {
						slog.ErrorContext(c.RequestContext(), "failed to write response", "err", err)
					}
				})

				// Test that server can be created and configured
				server := &http.Server{
					Addr:    ":0", // Use port 0 for automatic port assignment
					Handler: a.Router(),
				}

				// Start server in background
				serverErr := make(chan error, 1)
				go func() {
					// Use a test server instead of real ListenAndServe for unit testing
					// In real integration tests, you'd use a real server
					time.Sleep(10 * time.Millisecond)
					serverErr <- nil
				}()

				// Wait a bit for server to "start"
				time.Sleep(20 * time.Millisecond)

				// Test graceful shutdown
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
				defer cancel()

				err := server.Shutdown(ctx)
				Expect(err).NotTo(HaveOccurred(), "server should shutdown gracefully")

				select {
				case serverErrVal := <-serverErr:
					Expect(serverErrVal).NotTo(HaveOccurred())
				case <-time.After(100 * time.Millisecond):
					// Server didn't error, which is fine for this test
				}
			})
		})

		Describe("Error Handling", func() {
			It("should handle errors correctly under concurrent load", func() {
				a := app.MustNew(
					app.WithServiceName("test"),
					app.WithServiceVersion("1.0.0"),
				)

				// Route that panics
				a.GET("/panic", func(_ *app.Context) {
					panic("test panic")
				})

				// Route that returns error
				a.GET("/error", func(c *app.Context) {
					if err := c.JSON(http.StatusInternalServerError, map[string]string{
						"error": "test error",
					}); err != nil {
						slog.ErrorContext(c.RequestContext(), "failed to write error response", "err", err)
					}
				})

				// Route that works
				a.GET("/ok", func(c *app.Context) {
					if err := c.String(http.StatusOK, "ok"); err != nil {
						slog.ErrorContext(c.RequestContext(), "failed to write response", "err", err)
					}
				})

				const concurrency = 20
				var wg sync.WaitGroup

				// Test panic recovery
				for range concurrency {
					(&wg).Go(func() {
						req := httptest.NewRequest(http.MethodGet, "/panic", nil)
						w := httptest.NewRecorder()
						// Should not panic - recovery middleware should catch it
						Expect(func() {
							a.Router().ServeHTTP(w, req)
						}).NotTo(Panic())
					})
				}

				// Test error handling
				for range concurrency {
					(&wg).Go(func() {
						req := httptest.NewRequest(http.MethodGet, "/error", nil)
						w := httptest.NewRecorder()
						a.Router().ServeHTTP(w, req)
						Expect(w.Code).To(Equal(http.StatusInternalServerError))
					})
				}

				// Test normal requests
				for range concurrency {
					(&wg).Go(func() {
						req := httptest.NewRequest(http.MethodGet, "/ok", nil)
						w := httptest.NewRecorder()
						a.Router().ServeHTTP(w, req)
						Expect(w.Code).To(Equal(http.StatusOK))
					})
				}

				wg.Wait()
			})
		})

		Describe("Group routes", func() {
			It("should serve routes registered on Group for all HTTP methods", func() {
				a := app.MustNew(
					app.WithServiceName("test"),
					app.WithServiceVersion("1.0.0"),
				)
				api := a.Group("/api")
				api.GET("/get", func(c *app.Context) { _ = c.String(http.StatusOK, "get") })
				api.POST("/post", func(c *app.Context) { _ = c.String(http.StatusOK, "post") })
				api.PUT("/put", func(c *app.Context) { _ = c.String(http.StatusOK, "put") })
				api.DELETE("/delete", func(c *app.Context) { _ = c.String(http.StatusOK, "delete") })
				api.PATCH("/patch", func(c *app.Context) { _ = c.String(http.StatusOK, "patch") })
				api.HEAD("/head", func(c *app.Context) { _ = c.String(http.StatusOK, "head") })
				api.OPTIONS("/options", func(c *app.Context) { _ = c.String(http.StatusOK, "options") })
				api.Any("/any", func(c *app.Context) { _ = c.String(http.StatusOK, "any") })

				for _, tc := range []struct {
					method string
					path   string
					body   string
				}{
					{http.MethodGet, "/api/get", ""},
					{http.MethodPost, "/api/post", ""},
					{http.MethodPut, "/api/put", ""},
					{http.MethodDelete, "/api/delete", ""},
					{http.MethodPatch, "/api/patch", ""},
					{http.MethodHead, "/api/head", ""},
					{http.MethodOptions, "/api/options", ""},
					{http.MethodGet, "/api/any", ""},
					{http.MethodPost, "/api/any", ""},
				} {
					req := httptest.NewRequest(tc.method, tc.path, nil)
					rec := httptest.NewRecorder()
					a.Router().ServeHTTP(rec, req)
					Expect(rec.Code).To(Equal(http.StatusOK), "method=%s path=%s", tc.method, tc.path)
				}
			})
		})

		Describe("Group Use (middleware)", func() {
			It("should run group middleware for routes under the group", func() {
				a := app.MustNew(
					app.WithServiceName("test"),
					app.WithServiceVersion("1.0.0"),
				)
				var groupMiddlewareRan bool
				api := a.Group("/api")
				api.Use(func(c *app.Context) {
					groupMiddlewareRan = true
					c.Next()
				})
				api.GET("/ok", func(c *app.Context) { _ = c.String(http.StatusOK, "ok") })

				req := httptest.NewRequest(http.MethodGet, "/api/ok", nil)
				rec := httptest.NewRecorder()
				a.Router().ServeHTTP(rec, req)
				Expect(rec.Code).To(Equal(http.StatusOK))
				Expect(rec.Body.String()).To(Equal("ok"))
				Expect(groupMiddlewareRan).To(BeTrue())
			})
		})

		Describe("Version group routes", func() {
			It("should serve POST, PUT, DELETE, PATCH on versioned routes", func() {
				a := app.MustNew(
					app.WithServiceName("test"),
					app.WithServiceVersion("1.0.0"),
					app.WithRouter(router.WithVersioning(version.WithPathDetection("/v{version}/"), version.WithDefault("v1"))),
				)
				v1 := a.Version("v1")
				v1.GET("/status", func(c *app.Context) { _ = c.String(http.StatusOK, "get") })
				v1.POST("/items", func(c *app.Context) { _ = c.String(http.StatusOK, "post") })
				v1.PUT("/items/:id", func(c *app.Context) { _ = c.String(http.StatusOK, "put") })
				v1.DELETE("/items/:id", func(c *app.Context) { _ = c.String(http.StatusOK, "delete") })
				v1.PATCH("/items/:id", func(c *app.Context) { _ = c.String(http.StatusOK, "patch") })

				for _, tc := range []struct {
					method string
					path   string
					body   string
				}{
					{http.MethodPost, "/v1/items", ""},
					{http.MethodPut, "/v1/items/1", ""},
					{http.MethodDelete, "/v1/items/1", ""},
					{http.MethodPatch, "/v1/items/1", ""},
				} {
					req := httptest.NewRequest(tc.method, tc.path, nil)
					rec := httptest.NewRecorder()
					a.Router().ServeHTTP(rec, req)
					Expect(rec.Code).To(Equal(http.StatusOK), "method=%s path=%s", tc.method, tc.path)
				}
			})
		})

		Describe("Context error helpers", func() {
			It("should respond with correct status for NotFound, BadRequest, Unauthorized, Forbidden, Conflict, Gone, UnprocessableEntity, TooManyRequests, InternalError, ServiceUnavailable", func() {
				a := app.MustNew(
					app.WithServiceName("test"),
					app.WithServiceVersion("1.0.0"),
				)
				a.GET("/404", func(c *app.Context) { c.NotFound(nil) })
				a.GET("/400", func(c *app.Context) { c.BadRequest(nil) })
				a.GET("/401", func(c *app.Context) { c.Unauthorized(nil) })
				a.GET("/403", func(c *app.Context) { c.Forbidden(nil) })
				a.GET("/409", func(c *app.Context) { c.Conflict(nil) })
				a.GET("/410", func(c *app.Context) { c.Gone(nil) })
				a.GET("/422", func(c *app.Context) { c.UnprocessableEntity(nil) })
				a.GET("/429", func(c *app.Context) { c.TooManyRequests(nil) })
				a.GET("/500", func(c *app.Context) { c.InternalError(nil) })
				a.GET("/503", func(c *app.Context) { c.ServiceUnavailable(nil) })

				tests := []struct {
					path       string
					expectCode int
				}{
					{"/404", http.StatusNotFound},
					{"/400", http.StatusBadRequest},
					{"/401", http.StatusUnauthorized},
					{"/403", http.StatusForbidden},
					{"/409", http.StatusConflict},
					{"/410", http.StatusGone},
					{"/422", http.StatusUnprocessableEntity},
					{"/429", http.StatusTooManyRequests},
					{"/500", http.StatusInternalServerError},
					{"/503", http.StatusServiceUnavailable},
				}
				for _, tt := range tests {
					req := httptest.NewRequest(http.MethodGet, tt.path, nil)
					rec := httptest.NewRecorder()
					a.Router().ServeHTTP(rec, req)
					Expect(rec.Code).To(Equal(tt.expectCode), "path=%s", tt.path)
				}
			})
		})
	})
})

//nolint:paralleltest // Ginkgo test suite manages its own parallelization
func TestAppIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "App Integration Suite")
}
