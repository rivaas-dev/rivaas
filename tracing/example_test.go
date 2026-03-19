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

//go:build !integration

package tracing_test

import (
	"context"
	"fmt"
	"net/http"

	"go.opentelemetry.io/otel/attribute"

	"rivaas.dev/tracing"
)

// ExampleNew demonstrates creating a new tracing configuration.
func ExampleNew() {
	tracer, err := tracing.New(
		tracing.WithServiceName("my-service"),
		tracing.WithServiceVersion("1.0.0"),
		tracing.WithStdout(),
	)
	if err != nil {
		if _, printErr := fmt.Printf("Error: %v\n", err); printErr != nil {
			panic(printErr)
		}

		return
	}
	defer func() {
		if err = tracer.Shutdown(context.Background()); err != nil {
			panic(err)
		}
	}()

	if _, err = fmt.Printf("Tracing enabled: %v\n", tracer.IsEnabled()); err != nil {
		panic(err)
	}
	// Output: Tracing enabled: true
}

// ExampleMustNew demonstrates creating tracing configuration that panics on error.
func ExampleMustNew() {
	tracer := tracing.MustNew(
		tracing.WithServiceName("my-service"),
		tracing.WithStdout(),
	)
	defer func() {
		if err := tracer.Shutdown(context.Background()); err != nil {
			panic(err)
		}
	}()

	if _, err := fmt.Printf("Service: %s\n", tracer.ServiceName()); err != nil {
		panic(err)
	}
	// Output: Service: my-service
}

// ExampleStartSpan demonstrates creating and managing spans.
func ExampleTracer_StartSpan() {
	tracer := tracing.MustNew(
		tracing.WithServiceName("my-service"),
		tracing.WithStdout(),
	)
	defer func() {
		if err := tracer.Shutdown(context.Background()); err != nil {
			panic(err)
		}
	}()

	ctx := context.Background()
	_, span := tracer.StartSpan(ctx, "database-query")
	defer tracer.FinishSpan(span)

	tracer.SetSpanAttribute(span, "db.query", "SELECT * FROM users")
	tracer.SetSpanAttribute(span, "db.rows", 10)
	// Output:
}

// ExampleTracer_AddSpanEvent demonstrates adding events to spans.
func ExampleTracer_AddSpanEvent() {
	tracer := tracing.MustNew(
		tracing.WithServiceName("my-service"),
		tracing.WithStdout(),
	)
	defer func() {
		if err := tracer.Shutdown(context.Background()); err != nil {
			panic(err)
		}
	}()

	ctx := context.Background()
	ctx, span := tracer.StartSpan(ctx, "cache-operation")
	defer tracer.FinishSpan(span)

	tracer.AddSpanEvent(span, "cache_hit",
		attribute.String("key", "user:123"),
		attribute.Int("ttl", 3600),
	)

	_ = ctx // use ctx
	// Output:
}

// ExampleTracer_FinishSpan demonstrates ending a span with success.
func ExampleTracer_FinishSpan() {
	tracer := tracing.MustNew(
		tracing.WithServiceName("my-service"),
		tracing.WithStdout(),
	)
	defer func() {
		if err := tracer.Shutdown(context.Background()); err != nil {
			panic(err)
		}
	}()

	ctx, span := tracer.StartSpan(context.Background(), "operation")
	defer tracer.FinishSpan(span)
	_ = ctx
	// Output:
}

// ExampleTracer_FinishSpanWithError demonstrates ending a span with an error.
func ExampleTracer_FinishSpanWithError() {
	tracer := tracing.MustNew(
		tracing.WithServiceName("my-service"),
		tracing.WithStdout(),
	)
	defer func() {
		if err := tracer.Shutdown(context.Background()); err != nil {
			panic(err)
		}
	}()

	ctx, span := tracer.StartSpan(context.Background(), "operation")
	err := doWork(ctx)
	if err != nil {
		tracer.FinishSpanWithError(span, err)
		return
	}
	tracer.FinishSpan(span)
	// Output:
}

func doWork(context.Context) error {
	return nil
}

// ExampleTracer_WithSpan demonstrates running a function under a span.
func ExampleTracer_WithSpan() {
	tracer := tracing.MustNew(
		tracing.WithServiceName("my-service"),
		tracing.WithStdout(),
	)
	defer func() {
		if err := tracer.Shutdown(context.Background()); err != nil {
			panic(err)
		}
	}()

	err := tracer.WithSpan(context.Background(), "process-order", func(ctx context.Context) error {
		// Simulate work
		return nil
	})
	if err != nil {
		panic(err)
	}
	// Output:
}

// ExampleCopyTraceContext demonstrates propagating trace context to a goroutine.
func ExampleCopyTraceContext() {
	tracer := tracing.MustNew(
		tracing.WithServiceName("my-service"),
		tracing.WithStdout(),
	)
	defer func() {
		if err := tracer.Shutdown(context.Background()); err != nil {
			panic(err)
		}
	}()

	ctx, span := tracer.StartSpan(context.Background(), "parent")
	traceCtx := tracing.CopyTraceContext(ctx)
	go func() {
		_, childSpan := tracer.StartSpan(traceCtx, "async-job")
		defer tracer.FinishSpan(childSpan)
	}()
	tracer.FinishSpan(span)
	// Output:
}

// ExampleTracer_ExtractTraceContext demonstrates extracting trace context from headers.
func ExampleTracer_ExtractTraceContext() {
	ctx := context.Background()
	tracer := tracing.MustNew(
		tracing.WithServiceName("my-service"),
		tracing.WithStdout(),
	)
	defer func() {
		if err := tracer.Shutdown(ctx); err != nil {
			panic(err)
		}
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/api/users", nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")

	ctx = tracer.ExtractTraceContext(ctx, req.Header)
	ctx, span := tracer.StartSpan(ctx, "process-request")
	defer tracer.FinishSpan(span)
	// Output:
}

// ExampleWithSampleRate demonstrates configuring sampling rate.
func ExampleWithSampleRate() {
	tracer := tracing.MustNew(
		tracing.WithServiceName("my-service"),
		tracing.WithStdout(),
		tracing.WithSampleRate(0.1), // Sample 10% of requests
	)
	defer func() {
		if err := tracer.Shutdown(context.Background()); err != nil {
			panic(err)
		}
	}()

	if _, err := fmt.Printf("Service: %s\n", tracer.ServiceName()); err != nil {
		panic(err)
	}
	// Output: Service: my-service
}

// ExampleMiddleware demonstrates using middleware with path exclusion.
func ExampleMiddleware() {
	tracer := tracing.MustNew(
		tracing.WithServiceName("my-service"),
		tracing.WithStdout(),
	)
	defer func() {
		if err := tracer.Shutdown(context.Background()); err != nil {
			panic(err)
		}
	}()

	// Create handler with middleware options
	mux := http.NewServeMux()
	mux.HandleFunc("/api/users", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Use Middleware (returns error for invalid options; use MustMiddleware to panic)
	mw, err := tracing.Middleware(tracer,
		tracing.WithExcludePaths("/health", "/metrics", "/ready"),
		tracing.WithHeaders("X-Request-ID", "X-Correlation-ID"),
	)
	if err != nil {
		panic(err)
	}
	handler := mw(mux)

	// Use handler...
	_ = handler
	// Output:
}

// ExampleWithOTLP demonstrates configuring OTLP provider.
func ExampleWithOTLP() {
	tracer := tracing.MustNew(
		tracing.WithServiceName("my-service"),
		tracing.WithOTLP("localhost:4317", tracing.OTLPInsecure()),
	)
	defer func() {
		if err := tracer.Shutdown(context.Background()); err != nil {
			panic(err)
		}
	}()

	if _, err := fmt.Printf("Service: %s\n", tracer.ServiceName()); err != nil {
		panic(err)
	}
	// Output: Service: my-service
}

// ExampleWithOTLPHTTP demonstrates configuring OTLP HTTP provider.
func ExampleWithOTLPHTTP() {
	tracer := tracing.MustNew(
		tracing.WithServiceName("my-service"),
		tracing.WithOTLPHTTP("http://localhost:4318"),
	)
	defer func() {
		if err := tracer.Shutdown(context.Background()); err != nil {
			panic(err)
		}
	}()

	if _, err := fmt.Printf("Service: %s\n", tracer.ServiceName()); err != nil {
		panic(err)
	}
	// Output: Service: my-service
}
