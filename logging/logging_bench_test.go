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

package logging

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http/httptest"
	"testing"
	"time"
)

// Benchmark different handler types
func BenchmarkJSONHandler(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("benchmark message", "key", "value", "count", 42)
		}
	})
}

func BenchmarkTextHandler(b *testing.B) {
	logger := MustNew(WithTextHandler(), WithOutput(io.Discard))
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("benchmark message", "key", "value", "count", 42)
		}
	})
}

func BenchmarkConsoleHandler(b *testing.B) {
	logger := MustNew(WithConsoleHandler(), WithOutput(io.Discard))
	b.ResetTimer()
	b.ReportAllocs()
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
	b.ReportAllocs()
	for b.Loop() {
		logger.Info("message", "key", "value")
	}
}

func BenchmarkLogging_ManyAttrs(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
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
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("concurrent message",
				"goroutine", "test",
				"timestamp", time.Now().Unix(),
			)
		}
	})
}

// Benchmark context logger
func BenchmarkContextLogger(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		cl := NewContextLogger(ctx, logger)
		cl.Info("message", "key", "value")
	}
}

func BenchmarkContextLogger_Pooled(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			cl := NewContextLogger(ctx, logger)
			cl.Info("message", "key", "value")
		}
	})
}

// Benchmark batch logger
func BenchmarkBatchLogger(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	bl := NewBatchLogger(logger, 100, time.Second)
	b.Cleanup(func() { bl.Close() })

	b.ResetTimer()
	b.ReportAllocs()
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
	b.ReportAllocs()
	for b.Loop() {
		logger.Info("sampled message", "iteration", 1)
	}
}

// Benchmark error with stack
func BenchmarkErrorWithStack(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	err := errors.New("test error")

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		logger.ErrorWithStack("error occurred", err, true, "context", "test")
	}
}

func BenchmarkErrorWithoutStack(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	err := errors.New("test error")

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		logger.ErrorWithStack("error occurred", err, false, "context", "test")
	}
}

// Benchmark convenience methods
func BenchmarkLogRequest(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	req := httptest.NewRequest("GET", "/test?foo=bar", nil)

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		logger.LogRequest(req, "status", 200, "duration_ms", 45)
	}
}

func BenchmarkLogError(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	err := errors.New("test error")

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		logger.LogError(err, "operation failed", "retry", 3)
	}
}

func BenchmarkLogDuration(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	start := time.Now()

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		logger.LogDuration("operation completed", start, "rows", 100)
	}
}

// Benchmark different output types
func BenchmarkOutput_Stdout(b *testing.B) {
	logger := MustNew(WithJSONHandler())
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		logger.Info("message", "key", "value")
	}
}

func BenchmarkOutput_Discard(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		logger.Info("message", "key", "value")
	}
}

func BenchmarkOutput_Buffer(b *testing.B) {
	var buf bytes.Buffer
	logger := MustNew(WithJSONHandler(), WithOutput(&buf))
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		logger.Info("message", "key", "value")
	}
}

// Benchmark shutdown check (atomic vs mutex)
func BenchmarkShutdownCheck_Atomic(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = logger.isShuttingDown.Load()
		}
	})
}

// Benchmark pool operations
func BenchmarkPool_ContextLogger(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		cl := NewContextLogger(ctx, logger)
		_ = cl
	}
}

// Benchmark metrics tracking overhead
func BenchmarkMetricsOverhead(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		logger.Info("message with metrics", "key", "value")
	}
}

// Benchmark with vs without sampling
func BenchmarkNoSampling(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		logger.Info("message", "iteration", 1)
	}
}

func BenchmarkWithSampling(b *testing.B) {
	logger := MustNew(
		WithJSONHandler(),
		WithOutput(io.Discard),
		WithSampling(SamplingConfig{Initial: 10, Thereafter: 100}),
	)

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		logger.Info("message", "iteration", 1)
	}
}

// Benchmark validation overhead
func BenchmarkConfigValidation(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		cfg := defaultLogger()
		_ = cfg.Validate()
	}
}

// Benchmark SetLevel
func BenchmarkSetLevel(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))

	b.ResetTimer()
	b.ReportAllocs()
	toggle := false
	for b.Loop() {
		if toggle {
			logger.SetLevel(LevelDebug)
		} else {
			logger.SetLevel(LevelInfo)
		}
		toggle = !toggle
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
	b.ReportAllocs()
	for b.Loop() {
		_ = logger.DebugInfo()
	}
}

// Benchmark different log levels
func BenchmarkLevel_Debug(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard), WithDebugLevel())
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		logger.Debug("debug message", "key", "value")
	}
}

func BenchmarkLevel_Info(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		logger.Info("info message", "key", "value")
	}
}

func BenchmarkLevel_Warn(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		logger.Warn("warn message", "key", "value")
	}
}

func BenchmarkLevel_Error(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
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
	b.ReportAllocs()
	for b.Loop() {
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
	b.ReportAllocs()
	for b.Loop() {
		logger.Info("message without source", "key", "value")
	}
}

// Benchmark console handler type switch optimization
func BenchmarkConsoleHandler_StringAttr(b *testing.B) {
	logger := MustNew(WithConsoleHandler(), WithOutput(io.Discard))
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		logger.Info("message", "string_key", "string_value")
	}
}

func BenchmarkConsoleHandler_IntAttr(b *testing.B) {
	logger := MustNew(WithConsoleHandler(), WithOutput(io.Discard))
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		logger.Info("message", "int_key", 42)
	}
}

func BenchmarkConsoleHandler_MixedAttrs(b *testing.B) {
	logger := MustNew(WithConsoleHandler(), WithOutput(io.Discard))
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
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
	b.ReportAllocs()
	for b.Loop() {
		logger.Info("high frequency event", "iteration", 1)
	}
}

func BenchmarkBatchLogger_HighFrequency(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	bl := NewBatchLogger(logger, 1000, 100*time.Millisecond)
	b.Cleanup(func() { bl.Close() })

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		bl.Info("high frequency event", "iteration", 1)
	}
}

// Benchmark test helpers
func BenchmarkNewTestLogger(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
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
	b.ReportAllocs()
	for b.Loop() {
		_, _ = ParseJSONLogEntries(buf)
	}
}

// BenchmarkLoggerAccess_NoContention tests logger access contention.
func BenchmarkLoggerAccess_NoContention(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		_ = logger.Logger()
	}
}

func BenchmarkLoggerAccess_HighContention(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = logger.Logger()
		}
	})
}

// Benchmark pooling effectiveness under load
func BenchmarkPool_Effectiveness(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	ctx := context.Background()

	b.Run("without_pooling", func(b *testing.B) {
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				// Simulate creating new objects each time
				cl := NewContextLogger(ctx, logger)
				cl.Info("message", "key", "value")
			}
		})
	})

	b.Run("with_pooling", func(b *testing.B) {
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				cl := NewContextLogger(ctx, logger)
				cl.Info("message", "key", "value")
			}
		})
	})
}

// Benchmark cached context performance
func BenchmarkContextAllocation(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))

	b.Run("cached_context", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		for b.Loop() {
			logger.Info("message", "key", "value")
		}
	})

	b.Run("new_context_each_time", func(b *testing.B) {
		// Simulate old behavior (for comparison only)
		l := logger.Logger()
		b.ResetTimer()
		b.ReportAllocs()
		for b.Loop() {
			l.InfoContext(context.Background(), "message", "key", "value")
		}
	})
}

// Benchmark memory allocations
func BenchmarkAllocs_Simple(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		logger.Info("message")
	}
}

func BenchmarkAllocs_WithAttrs(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		logger.Info("message", "key1", "value1", "key2", 42)
	}
}

func BenchmarkAllocs_ErrorWithStack(b *testing.B) {
	logger := MustNew(WithJSONHandler(), WithOutput(io.Discard))
	err := errors.New("test error")

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		logger.ErrorWithStack("error", err, true)
	}
}
