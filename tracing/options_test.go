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

package tracing

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// TestWithServiceName tests the WithServiceName option.
func TestWithServiceName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		serviceName string
		wantErr     bool
		errContains string
	}{
		{
			name:        "valid service name",
			serviceName: "my-service",
			wantErr:     false,
		},
		{
			name:        "service name with spaces",
			serviceName: "my service",
			wantErr:     false,
		},
		{
			name:        "service name with special characters",
			serviceName: "my-service_v1.0",
			wantErr:     false,
		},
		{
			name:        "empty service name",
			serviceName: "",
			wantErr:     true,
			errContains: "serviceName: cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tracer, err := New(WithServiceName(tt.serviceName))
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, tracer)
			t.Cleanup(func() { _ = tracer.Shutdown(t.Context()) })

			assert.Equal(t, tt.serviceName, tracer.ServiceName())
		})
	}
}

// TestWithServiceVersion tests the WithServiceVersion option.
func TestWithServiceVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		serviceVersion string
		wantErr        bool
		errContains    string
	}{
		{
			name:           "valid semantic version",
			serviceVersion: "v1.2.3",
			wantErr:        false,
		},
		{
			name:           "version without v prefix",
			serviceVersion: "1.2.3",
			wantErr:        false,
		},
		{
			name:           "prerelease version",
			serviceVersion: "v1.0.0-alpha.1",
			wantErr:        false,
		},
		{
			name:           "empty version",
			serviceVersion: "",
			wantErr:        true,
			errContains:    "serviceVersion: cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tracer, err := New(
				WithServiceName("test"),
				WithServiceVersion(tt.serviceVersion),
			)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, tracer)
			t.Cleanup(func() { _ = tracer.Shutdown(t.Context()) })

			assert.Equal(t, tt.serviceVersion, tracer.ServiceVersion())
		})
	}
}

// TestWithSampleRate tests the WithSampleRate option.
func TestWithSampleRate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		rate         float64
		expectedRate float64
	}{
		{
			name:         "100 percent",
			rate:         1.0,
			expectedRate: 1.0,
		},
		{
			name:         "50 percent",
			rate:         0.5,
			expectedRate: 0.5,
		},
		{
			name:         "0 percent",
			rate:         0.0,
			expectedRate: 0.0,
		},
		{
			name:         "10 percent",
			rate:         0.1,
			expectedRate: 0.1,
		},
		{
			name:         "negative rate clamped to 0",
			rate:         -0.5,
			expectedRate: 0.0,
		},
		{
			name:         "rate above 1 clamped to 1",
			rate:         1.5,
			expectedRate: 1.0,
		},
		{
			name:         "extremely negative rate clamped to 0",
			rate:         -999.9,
			expectedRate: 0.0,
		},
		{
			name:         "extremely high rate clamped to 1",
			rate:         999.9,
			expectedRate: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tracer := MustNew(
				WithServiceName("test"),
				WithSampleRate(tt.rate),
			)
			t.Cleanup(func() { _ = tracer.Shutdown(t.Context()) })

			assert.Equal(t, tt.expectedRate, tracer.sampleRate) //nolint:testifylint // exact sample rate comparison
		})
	}
}

// TestWithCustomTracer tests the WithCustomTracer option.
// Note: WithCustomTracer should be used with WithTracerProvider to ensure
// the custom tracer is preserved (otherwise initializeProvider creates a new tracer).
func TestWithCustomTracer(t *testing.T) {
	t.Parallel()

	// Create a noop tracer provider and tracer
	noopProvider := noop.NewTracerProvider()
	customTracer := noopProvider.Tracer("custom-tracer")

	tracer := MustNew(
		WithServiceName("test"),
		WithTracerProvider(noopProvider),
		WithCustomTracer(customTracer),
	)
	t.Cleanup(func() { _ = tracer.Shutdown(t.Context()) })

	assert.Equal(t, customTracer, tracer.tracer)
	assert.True(t, tracer.IsEnabled())
}

// TestWithCustomPropagator tests the WithCustomPropagator option.
func TestWithCustomPropagator(t *testing.T) {
	t.Parallel()

	customPropagator := propagation.TraceContext{}

	tracer := MustNew(
		WithServiceName("test"),
		WithCustomPropagator(customPropagator),
	)
	t.Cleanup(func() { _ = tracer.Shutdown(t.Context()) })

	assert.NotNil(t, tracer.GetPropagator())
}

// TestWithEventHandler tests the WithEventHandler option.
func TestWithEventHandler(t *testing.T) {
	t.Parallel()

	var capturedEvents []Event
	handler := func(e Event) {
		capturedEvents = append(capturedEvents, e)
	}

	tracer := MustNew(
		WithServiceName("test"),
		WithEventHandler(handler),
	)
	t.Cleanup(func() { _ = tracer.Shutdown(t.Context()) })

	// Trigger an event by calling internal emit methods
	tracer.emitInfo("test message", "key", "value")

	assert.Len(t, capturedEvents, 1)
	assert.Equal(t, EventInfo, capturedEvents[0].Type)
	assert.Equal(t, "test message", capturedEvents[0].Message)
}

// TestWithLogger tests the WithLogger option.
func TestWithLogger(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	tracer := MustNew(
		WithServiceName("test"),
		WithLogger(logger),
	)
	t.Cleanup(func() { _ = tracer.Shutdown(t.Context()) })

	assert.NotNil(t, tracer.eventHandler)
}

// TestWithLogger_NilLogger tests WithLogger with nil logger.
func TestWithLogger_NilLogger(t *testing.T) {
	t.Parallel()

	tracer := MustNew(
		WithServiceName("test"),
		WithLogger(nil),
	)
	t.Cleanup(func() { _ = tracer.Shutdown(t.Context()) })

	// Should not panic when emitting events with nil logger
	tracer.emitInfo("test message")
	tracer.emitWarning("warning message")
	tracer.emitError("error message")
	tracer.emitDebug("debug message")
}

// TestWithSpanStartHook tests the WithSpanStartHook option.
func TestWithSpanStartHook(t *testing.T) {
	t.Parallel()

	var hookCalled bool
	var capturedMethod string

	hook := func(_ context.Context, _ trace.Span, req *http.Request) {
		hookCalled = true
		capturedMethod = req.Method
	}

	tracer := MustNew(
		WithServiceName("test"),
		WithSpanStartHook(hook),
	)
	t.Cleanup(func() { _ = tracer.Shutdown(t.Context()) })

	assert.NotNil(t, tracer.spanStartHook)

	// Note: Hook is called during StartRequestSpan, not StartSpan
	// The actual invocation is tested in middleware_test.go
	_ = hookCalled
	_ = capturedMethod
}

// TestWithSpanFinishHook tests the WithSpanFinishHook option.
func TestWithSpanFinishHook(t *testing.T) {
	t.Parallel()

	var hookCalled bool
	var capturedStatus int

	hook := func(_ trace.Span, statusCode int) {
		hookCalled = true
		capturedStatus = statusCode
	}

	tracer := MustNew(
		WithServiceName("test"),
		WithSpanFinishHook(hook),
	)
	t.Cleanup(func() { _ = tracer.Shutdown(t.Context()) })

	assert.NotNil(t, tracer.spanFinishHook)

	// Note: Hook is called during FinishRequestSpan
	// The actual invocation is tested in middleware_test.go
	_ = hookCalled
	_ = capturedStatus
}

// TestWithGlobalTracerProvider tests the WithGlobalTracerProvider option.
func TestWithGlobalTracerProvider(t *testing.T) {
	t.Parallel()

	tracer := MustNew(
		WithServiceName("test"),
		WithGlobalTracerProvider(),
	)
	t.Cleanup(func() { _ = tracer.Shutdown(t.Context()) })

	assert.True(t, tracer.registerGlobal)
}

// TestProviderOptions tests the provider configuration options.
func TestProviderOptions(t *testing.T) {
	t.Parallel()

	t.Run("WithStdout", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(
			WithServiceName("test"),
			WithStdout(),
		)
		t.Cleanup(func() { _ = tracer.Shutdown(t.Context()) })

		assert.Equal(t, StdoutProvider, tracer.GetProvider())
	})

	t.Run("WithNoop", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(
			WithServiceName("test"),
			WithNoop(),
		)
		t.Cleanup(func() { _ = tracer.Shutdown(t.Context()) })

		assert.Equal(t, NoopProvider, tracer.GetProvider())
	})

	t.Run("DefaultProvider", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(
			WithServiceName("test"),
		)
		t.Cleanup(func() { _ = tracer.Shutdown(t.Context()) })

		// Default should be noop
		assert.Equal(t, NoopProvider, tracer.GetProvider())
	})
}

// TestOTLPOptions tests OTLP-specific options.
func TestOTLPOptions(t *testing.T) {
	t.Parallel()

	t.Run("WithOTLP_BasicEndpoint", func(t *testing.T) {
		t.Parallel()

		tracer, err := New(
			WithServiceName("test"),
			WithOTLP("localhost:4317"),
		)
		// May fail if OTLP collector not running
		if err == nil {
			t.Cleanup(func() { _ = tracer.Shutdown(t.Context()) })
			assert.Equal(t, OTLPProvider, tracer.GetProvider())
		}
	})

	t.Run("WithOTLP_Insecure", func(t *testing.T) {
		t.Parallel()

		tracer, err := New(
			WithServiceName("test"),
			WithOTLP("localhost:4317", OTLPInsecure()),
		)
		// May fail if OTLP collector not running
		if err == nil {
			t.Cleanup(func() { _ = tracer.Shutdown(t.Context()) })
			assert.Equal(t, OTLPProvider, tracer.GetProvider())
			assert.True(t, tracer.otlpInsecure)
		}
	})

	t.Run("WithOTLPHTTP", func(t *testing.T) {
		t.Parallel()

		tracer, err := New(
			WithServiceName("test"),
			WithOTLPHTTP("http://localhost:4318"),
		)
		// May fail if OTLP collector not running
		if err == nil {
			t.Cleanup(func() { _ = tracer.Shutdown(t.Context()) })
			assert.Equal(t, OTLPHTTPProvider, tracer.GetProvider())
		}
	})
}

// TestMultipleProviders tests that configuring multiple providers returns an error.
func TestMultipleProviders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		opts     []Option
		contains string
	}{
		{
			name: "stdout then noop",
			opts: []Option{
				WithServiceName("test"),
				WithStdout(),
				WithNoop(),
			},
			contains: "multiple providers configured",
		},
		{
			name: "noop then stdout",
			opts: []Option{
				WithServiceName("test"),
				WithNoop(),
				WithStdout(),
			},
			contains: "multiple providers configured",
		},
		{
			name: "stdout then otlp",
			opts: []Option{
				WithServiceName("test"),
				WithStdout(),
				WithOTLP("localhost:4317"),
			},
			contains: "multiple providers configured",
		},
		{
			name: "noop then otlp-http",
			opts: []Option{
				WithServiceName("test"),
				WithNoop(),
				WithOTLPHTTP("http://localhost:4318"),
			},
			contains: "multiple providers configured",
		},
		{
			name: "three providers",
			opts: []Option{
				WithServiceName("test"),
				WithStdout(),
				WithOTLP("localhost:4317"),
				WithNoop(),
			},
			contains: "multiple providers configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tracer, err := New(tt.opts...)
			require.Error(t, err)
			assert.Nil(t, tracer)
			assert.Contains(t, err.Error(), tt.contains)
		})
	}
}

// TestDefaultEventHandler tests the DefaultEventHandler function.
func TestDefaultEventHandler(t *testing.T) {
	t.Parallel()

	t.Run("WithLogger", func(t *testing.T) {
		t.Parallel()

		logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
		handler := DefaultEventHandler(logger)

		require.NotNil(t, handler)

		// Should not panic for any event type
		handler(Event{Type: EventError, Message: "error", Args: []any{"key", "value"}})
		handler(Event{Type: EventWarning, Message: "warning", Args: nil})
		handler(Event{Type: EventInfo, Message: "info", Args: nil})
		handler(Event{Type: EventDebug, Message: "debug", Args: nil})
	})

	t.Run("WithNilLogger", func(t *testing.T) {
		t.Parallel()

		handler := DefaultEventHandler(nil)
		require.NotNil(t, handler)

		// Should not panic - no-op handler
		handler(Event{Type: EventError, Message: "error", Args: nil})
		handler(Event{Type: EventWarning, Message: "warning", Args: nil})
	})
}

// TestOptionsCombination tests various option combinations.
func TestOptionsCombination(t *testing.T) {
	t.Parallel()

	t.Run("AllCommonOptions", func(t *testing.T) {
		t.Parallel()

		var events []Event
		eventHandler := func(e Event) {
			events = append(events, e)
		}

		tracer := MustNew(
			WithServiceName("combined-test"),
			WithServiceVersion("v2.0.0"),
			WithSampleRate(0.5),
			WithNoop(),
			WithEventHandler(eventHandler),
		)
		t.Cleanup(func() { _ = tracer.Shutdown(t.Context()) })

		assert.Equal(t, "combined-test", tracer.ServiceName())
		assert.Equal(t, "v2.0.0", tracer.ServiceVersion())
		assert.Equal(t, 0.5, tracer.sampleRate) //nolint:testifylint // exact sample rate comparison
		assert.Equal(t, NoopProvider, tracer.GetProvider())
		assert.NotNil(t, tracer.eventHandler)
	})

	t.Run("OverrideDefaults", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(
			WithServiceName("first-name"),
			WithServiceName("second-name"), // Should override
			WithSampleRate(0.1),
			WithSampleRate(0.9), // Should override
		)
		t.Cleanup(func() { _ = tracer.Shutdown(t.Context()) })

		assert.Equal(t, "second-name", tracer.ServiceName())
		assert.Equal(t, 0.9, tracer.sampleRate) //nolint:testifylint // exact sample rate comparison
	})
}
