package logging

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Benchmark different handler types
func BenchmarkJSONHandler(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("benchmark message", "key", "value", "count", 42)
		}
	})
}

func BenchmarkTextHandler(b *testing.B) {
	logger := MustNew(WithTextHandler(), WithOutput(io.Discard))
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("benchmark message", "key", "value", "count", 42)
		}
	})
}

func BenchmarkConsoleHandler(b *testing.B) {
	logger := MustNew(WithConsoleHandler(), WithOutput(io.Discard))
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("benchmark message", "key", "value", "count", 42)
		}
	})
}

// Benchmark with different numbers of attributes
func BenchmarkLogging_FewAttrs(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("message", "key", "value")
	}
}

func BenchmarkLogging_ManyAttrs(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("message",
			"key1", "value1",
			"key2", "value2",
			"key3", "value3",
			"key4", "value4",
			"key5", "value5",
			"key6", "value6",
			"key7", "value7",
			"key8", "value8",
		)
	}
}

// Benchmark concurrent logging
func BenchmarkConcurrentLogging(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("concurrent message",
				"goroutine", "test",
				"timestamp", time.Now().Unix(),
			)
		}
	})
}

// Benchmark HTTP middleware
func BenchmarkMiddleware_Basic(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	mw := Middleware(logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	wrapped := mw(handler)
	req := httptest.NewRequest("GET", "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		wrapped.ServeHTTP(w, req)
	}
}

func BenchmarkMiddleware_WithHeaders(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	mw := Middleware(logger, WithLogHeaders(true))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := mw(handler)
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("User-Agent", "Test/1.0")
	req.Header.Set("Accept", "application/json")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		wrapped.ServeHTTP(w, req)
	}
}

func BenchmarkMiddleware_SkipPath(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	mw := Middleware(logger, WithSkipPaths("/health"))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := mw(handler)
	req := httptest.NewRequest("GET", "/health", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		wrapped.ServeHTTP(w, req)
	}
}

// Benchmark context logger
func BenchmarkContextLogger(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cl := NewContextLogger(logger, ctx)
		cl.Info("message", "key", "value")
	}
}

func BenchmarkContextLogger_Pooled(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			cl := contextLoggerPool.Get().(*ContextLogger)
			cl.reset(logger, ctx)
			cl.Info("message", "key", "value")
			contextLoggerPool.Put(cl)
		}
	})
}

// Benchmark batch logger
func BenchmarkBatchLogger(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	bl := NewBatchLogger(logger, 100, time.Second)
	defer bl.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			bl.Info("test message", "key", "value", "count", 42)
		}
	})
}

// Benchmark sampling
func BenchmarkSampledLogging(b *testing.B) {
	logger := MustNew(
		WithJSONHandler(),
		WithOutput(io.Discard),
		WithSampling(SamplingConfig{
			Initial:    10,
			Thereafter: 100,
		}),
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("sampled message", "iteration", i)
	}
}

// Benchmark error with stack
func BenchmarkErrorWithStack(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	err := errors.New("test error")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.ErrorWithStack("error occurred", err, true, "context", "test")
	}
}

func BenchmarkErrorWithoutStack(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	err := errors.New("test error")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.ErrorWithStack("error occurred", err, false, "context", "test")
	}
}

// Benchmark convenience methods
func BenchmarkLogRequest(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	req := httptest.NewRequest("GET", "/test?foo=bar", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.LogRequest(req, "status", 200, "duration_ms", 45)
	}
}

func BenchmarkLogError(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	err := errors.New("test error")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.LogError(err, "operation failed", "retry", 3)
	}
}

func BenchmarkLogDuration(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	start := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.LogDuration("operation completed", start, "rows", 100)
	}
}

// Benchmark different output types
func BenchmarkOutput_Stdout(b *testing.B) {
	logger := MustNew(WithJSONHandler())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("message", "key", "value")
	}
}

func BenchmarkOutput_Discard(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("message", "key", "value")
	}
}

func BenchmarkOutput_Buffer(b *testing.B) {
	var buf bytes.Buffer
	logger := MustNew(WithJSONHandler(), WithOutput(&buf))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("message", "key", "value")
	}
}

// Benchmark shutdown check (atomic vs mutex)
func BenchmarkShutdownCheck_Atomic(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = logger.isShuttingDown.Load()
		}
	})
}

// Benchmark pool operations
func BenchmarkPool_ResponseWriter(b *testing.B) {
	w := httptest.NewRecorder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rw := responseWriterPool.Get().(*responseWriter)
		rw.reset(w)
		responseWriterPool.Put(rw)
	}
}

func BenchmarkPool_ContextLogger(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cl := contextLoggerPool.Get().(*ContextLogger)
		cl.reset(logger, ctx)
		contextLoggerPool.Put(cl)
	}
}

func BenchmarkPool_AttrSlice(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		attrsPtr := attrSlicePool.Get().(*[]any)
		*attrsPtr = (*attrsPtr)[:0]
		attrSlicePool.Put(attrsPtr)
	}
}

// Benchmark metrics tracking overhead
func BenchmarkMetricsOverhead(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("message with metrics", "key", "value")
	}
}

// Benchmark with vs without sampling
func BenchmarkNoSampling(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("message", "iteration", i)
	}
}

func BenchmarkWithSampling(b *testing.B) {
	logger := MustNew(
		WithJSONHandler(),
		WithOutput(io.Discard),
		WithSampling(SamplingConfig{Initial: 10, Thereafter: 100}),
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("message", "iteration", i)
	}
}

// Benchmark validation overhead
func BenchmarkConfigValidation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg := defaultConfig()
		_ = cfg.Validate()
	}
}

// Benchmark SetLevel
func BenchmarkSetLevel(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i%2 == 0 {
			logger.SetLevel(LevelDebug)
		} else {
			logger.SetLevel(LevelInfo)
		}
	}
}

// Benchmark DebugInfo
func BenchmarkDebugInfo(b *testing.B) {
	logger := MustNew(
		WithJSONHandler(),
		WithOutput(io.Discard),
		WithSampling(SamplingConfig{Initial: 10, Thereafter: 100}),
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = logger.DebugInfo()
	}
}

// Benchmark metrics retrieval
func BenchmarkGetMetrics(b *testing.B) {
	// Benchmark removed - metrics feature has been removed from the package
	b.Skip("Metrics feature has been removed")
}

// Benchmark different log levels
func BenchmarkLevel_Debug(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard), WithDebugLevel())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Debug("debug message", "key", "value")
	}
}

func BenchmarkLevel_Info(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("info message", "key", "value")
	}
}

func BenchmarkLevel_Warn(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Warn("warn message", "key", "value")
	}
}

func BenchmarkLevel_Error(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Error("error message", "key", "value")
	}
}

// Benchmark with source enabled
func BenchmarkWithSource(b *testing.B) {
	logger := MustNew(
		WithJSONHandler(),
		WithOutput(io.Discard),
		WithSource(true),
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("message with source", "key", "value")
	}
}

func BenchmarkWithoutSource(b *testing.B) {
	logger := MustNew(
		WithJSONHandler(),
		WithOutput(io.Discard),
		WithSource(false),
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("message without source", "key", "value")
	}
}

// Benchmark console handler type switch optimization
func BenchmarkConsoleHandler_StringAttr(b *testing.B) {
	logger := MustNew(WithConsoleHandler(), WithOutput(io.Discard))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("message", "string_key", "string_value")
	}
}

func BenchmarkConsoleHandler_IntAttr(b *testing.B) {
	logger := MustNew(WithConsoleHandler(), WithOutput(io.Discard))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("message", "int_key", 42)
	}
}

func BenchmarkConsoleHandler_MixedAttrs(b *testing.B) {
	logger := MustNew(WithConsoleHandler(), WithOutput(io.Discard))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("message",
			"string", "value",
			"int", 42,
			"int64", int64(100),
			"bool", true,
			"duration", time.Second,
		)
	}
}

// Benchmark batch logger vs regular logger
func BenchmarkRegularLogger_HighFrequency(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("high frequency event", "iteration", i)
	}
}

func BenchmarkBatchLogger_HighFrequency(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	bl := NewBatchLogger(logger, 1000, 100*time.Millisecond)
	defer bl.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bl.Info("high frequency event", "iteration", i)
	}
}

// Benchmark test helpers
func BenchmarkNewTestLogger(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger, buf := NewTestLogger()
		logger.Info("test message")
		_ = buf
	}
}

func BenchmarkParseLogEntries(b *testing.B) {
	logger, buf := NewTestLogger()
	for i := 0; i < 100; i++ {
		logger.Info("test message", "iteration", i, "timestamp", time.Now())
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ParseJSONLogEntries(buf)
	}
}

// Benchmark logger access contention (tests atomic.Pointer performance)
func BenchmarkLoggerAccess_NoContention(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = logger.Logger()
	}
}

func BenchmarkLoggerAccess_HighContention(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = logger.Logger()
		}
	})
}

// Benchmark middleware under realistic concurrent load
func BenchmarkMiddleware_Concurrent(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	mw := Middleware(logger)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate minimal work
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	wrapped := mw(handler)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		req := httptest.NewRequest("GET", "/test?foo=bar", nil)
		req.Header.Set("User-Agent", "Benchmark/1.0")
		for pb.Next() {
			w := httptest.NewRecorder()
			wrapped.ServeHTTP(w, req)
		}
	})
}

func BenchmarkMiddleware_ConcurrentWithHeaders(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	mw := Middleware(logger, WithLogHeaders(true))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	wrapped := mw(handler)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer token")
		req.Header.Set("User-Agent", "Benchmark/1.0")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Accept-Language", "en-US")
		req.Header.Set("Cache-Control", "no-cache")
		for pb.Next() {
			w := httptest.NewRecorder()
			wrapped.ServeHTTP(w, req)
		}
	})
}

// Benchmark pooling effectiveness under load
func BenchmarkPool_Effectiveness(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	ctx := context.Background()

	b.Run("without_pooling", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				// Simulate creating new objects each time
				cl := NewContextLogger(logger, ctx)
				cl.Info("message", "key", "value")
			}
		})
	})

	b.Run("with_pooling", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				cl := contextLoggerPool.Get().(*ContextLogger)
				cl.reset(logger, ctx)
				cl.Info("message", "key", "value")
				contextLoggerPool.Put(cl)
			}
		})
	})
}

// Benchmark cached context performance
func BenchmarkContextAllocation(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))

	b.Run("cached_context", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.Info("message", "key", "value")
		}
	})

	b.Run("new_context_each_time", func(b *testing.B) {
		// Simulate old behavior (for comparison only)
		l := logger.Logger()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.InfoContext(context.Background(), "message", "key", "value")
		}
	})
}

// Benchmark memory allocations
func BenchmarkAllocs_Simple(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("message")
	}
}

func BenchmarkAllocs_WithAttrs(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("message", "key1", "value1", "key2", 42)
	}
}

func BenchmarkAllocs_ErrorWithStack(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	err := errors.New("test error")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.ErrorWithStack("error", err, true)
	}
}
