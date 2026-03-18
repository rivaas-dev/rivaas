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
			t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

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
			t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

			assert.Equal(t, tt.serviceVersion, tracer.ServiceVersion())
		})
	}
}

// TestWithSampleRate tests the WithSampleRate option.
func TestWithSampleRate(t *testing.T) {
	t.Parallel()

	validTests := []struct {
		name         string
		rate         float64
		expectedRate float64
	}{
		{"100 percent", 1.0, 1.0},
		{"50 percent", 0.5, 0.5},
		{"0 percent", 0.0, 0.0},
		{"10 percent", 0.1, 0.1},
	}

	for _, tt := range validTests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tracer := MustNew(
				WithServiceName("test"),
				WithSampleRate(tt.rate),
			)
			t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

			assert.Equal(t, tt.expectedRate, tracer.sampleRate) //nolint:testifylint // exact sample rate comparison
		})
	}

	invalidRates := []struct {
		name string
		rate float64
	}{
		{"negative rate", -0.5},
		{"rate above 1", 1.5},
		{"extremely negative", -999.9},
		{"extremely high", 999.9},
	}

	for _, tc := range invalidRates {
		tc := tc
		t.Run("New_rejects_"+tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := New(WithServiceName("test"), WithSampleRate(tc.rate))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "sampleRate")
		})
		t.Run("MustNew_panics_"+tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Panics(t, func() {
				MustNew(WithServiceName("test"), WithSampleRate(tc.rate))
			})
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
	t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

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
	t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

	assert.NotNil(t, tracer.GetPropagator())
}

// TestWithLogger tests the WithLogger option.
func TestWithLogger(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	tracer := MustNew(
		WithServiceName("test"),
		WithLogger(logger),
	)
	t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

	assert.NotNil(t, tracer.logger)
}

// TestWithLogger_NilLogger tests WithLogger with nil logger (uses discard logger internally).
func TestWithLogger_NilLogger(t *testing.T) {
	t.Parallel()

	tracer := MustNew(
		WithServiceName("test"),
		WithLogger(nil),
	)
	t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

	// Logger is never nil (discard logger when nil passed); should not panic
	require.NotNil(t, tracer.logger)
	tracer.logger.Info("test message")
	tracer.logger.Warn("warning message")
	tracer.logger.Error("error message")
	tracer.logger.Debug("debug message")
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
	t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

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
	t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

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
	t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

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
		t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

		assert.Equal(t, StdoutProvider, tracer.GetProvider())
	})

	t.Run("WithNoop", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(
			WithServiceName("test"),
			WithNoop(),
		)
		t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

		assert.Equal(t, NoopProvider, tracer.GetProvider())
	})

	t.Run("DefaultProvider", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(
			WithServiceName("test"),
		)
		t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

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
			t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup
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
			t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup
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
			t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup
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

// TestOptionsCombination tests various option combinations.
func TestOptionsCombination(t *testing.T) {
	t.Parallel()

	t.Run("AllCommonOptions", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(
			WithServiceName("combined-test"),
			WithServiceVersion("v2.0.0"),
			WithSampleRate(0.5),
			WithNoop(),
			WithLogger(slog.Default()),
		)
		t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

		assert.Equal(t, "combined-test", tracer.ServiceName())
		assert.Equal(t, "v2.0.0", tracer.ServiceVersion())
		assert.Equal(t, 0.5, tracer.sampleRate) //nolint:testifylint // exact sample rate comparison
		assert.Equal(t, NoopProvider, tracer.GetProvider())
		assert.NotNil(t, tracer.logger)
	})

	t.Run("OverrideDefaults", func(t *testing.T) {
		t.Parallel()

		tracer := MustNew(
			WithServiceName("first-name"),
			WithServiceName("second-name"), // Should override
			WithSampleRate(0.1),
			WithSampleRate(0.9), // Should override
		)
		t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

		assert.Equal(t, "second-name", tracer.ServiceName())
		assert.Equal(t, 0.9, tracer.sampleRate) //nolint:testifylint // exact sample rate comparison
	})
}
