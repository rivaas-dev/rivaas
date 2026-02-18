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
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

// TestInitNoopProvider_WithCustomProviderAndGlobalRegistration covers initNoopProvider with custom provider and registerGlobal.
func TestInitNoopProvider_WithCustomProviderAndGlobalRegistration(t *testing.T) {
	t.Parallel()

	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	require.NoError(t, err)
	customTP := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("test"),
		)),
	)
	t.Cleanup(func() { customTP.Shutdown(context.Background()) }) //nolint:errcheck // Test cleanup

	tracer, err := New(
		WithTracerProvider(customTP),
		WithGlobalTracerProvider(),
		WithNoop(),
		WithServiceName("test"),
	)
	require.NoError(t, err)
	require.NotNil(t, tracer)
	t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

	assert.True(t, tracer.customTracerProvider)
	_, span := tracer.StartSpan(t.Context(), "test")
	require.NotNil(t, span)
	tracer.FinishSpan(span, 200)
}

// TestInitStdoutProvider_WithCustomProvider covers initStdoutProvider with custom provider.
func TestInitStdoutProvider_WithCustomProvider(t *testing.T) {
	t.Parallel()

	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	require.NoError(t, err)
	customTP := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("test"),
		)),
	)
	t.Cleanup(func() { customTP.Shutdown(context.Background()) }) //nolint:errcheck // Test cleanup

	tracer, err := New(
		WithTracerProvider(customTP),
		WithStdout(),
		WithServiceName("test"),
	)
	require.NoError(t, err)
	require.NotNil(t, tracer)
	t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

	assert.True(t, tracer.customTracerProvider)
	_, span := tracer.StartSpan(t.Context(), "test")
	require.NotNil(t, span)
	tracer.FinishSpan(span, 200)
}

// TestInitStdoutProvider_SkipsGlobalRegistration covers initStdoutProvider "Skipping global" branch.
func TestInitStdoutProvider_SkipsGlobalRegistration(t *testing.T) {
	t.Parallel()

	var debugMessages []string
	handler := func(e Event) {
		if e.Type == EventDebug {
			debugMessages = append(debugMessages, e.Message)
		}
	}

	tracer, err := New(
		WithServiceName("test"),
		WithStdout(),
		WithEventHandler(handler),
	)
	require.NoError(t, err)
	require.NotNil(t, tracer)
	t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

	var found bool
	if slices.Contains(debugMessages, "Skipping global tracer provider registration") {
		found = true
	}
	assert.True(t, found, "expected debug message for skipping global registration")
}

// TestInitOTLPProvider_WithCustomProvider covers initOTLPProvider with custom provider.
func TestInitOTLPProvider_WithCustomProvider(t *testing.T) {
	t.Parallel()

	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	require.NoError(t, err)
	customTP := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("test"),
		)),
	)
	t.Cleanup(func() { customTP.Shutdown(context.Background()) }) //nolint:errcheck // Test cleanup

	tracer, err := New(
		WithTracerProvider(customTP),
		WithOTLP("localhost:4317", OTLPInsecure()),
		WithServiceName("test"),
	)
	require.NoError(t, err)
	require.NotNil(t, tracer)
	t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	err = tracer.Start(ctx)
	require.NoError(t, err)

	assert.True(t, tracer.customTracerProvider)
	_, span := tracer.StartSpan(t.Context(), "test")
	require.NotNil(t, span)
	tracer.FinishSpan(span, 200)
}

// TestInitOTLPHTTPProvider_WithCustomProvider covers initOTLPHTTPProvider with custom provider.
func TestInitOTLPHTTPProvider_WithCustomProvider(t *testing.T) {
	t.Parallel()

	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	require.NoError(t, err)
	customTP := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("test"),
		)),
	)
	t.Cleanup(func() { customTP.Shutdown(context.Background()) }) //nolint:errcheck // Test cleanup

	tracer, err := New(
		WithTracerProvider(customTP),
		WithOTLPHTTP("http://localhost:4318"),
		WithServiceName("test"),
	)
	require.NoError(t, err)
	require.NotNil(t, tracer)
	t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	err = tracer.Start(ctx)
	require.NoError(t, err)

	assert.True(t, tracer.customTracerProvider)
	_, span := tracer.StartSpan(t.Context(), "test")
	require.NotNil(t, span)
	tracer.FinishSpan(span, 200)
}

// TestInitOTLPHTTPProvider_StripsHttpPrefixAndPath covers endpoint parsing (http prefix and path stripping).
func TestInitOTLPHTTPProvider_StripsHttpPrefixAndPath(t *testing.T) {
	t.Parallel()

	tracer, err := New(
		WithServiceName("test"),
		WithOTLPHTTP("http://localhost:4318/v1/traces"),
	)
	require.NoError(t, err)
	require.NotNil(t, tracer)
	t.Cleanup(func() { tracer.Shutdown(t.Context()) }) //nolint:errcheck // Test cleanup

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	// Start will call initOTLPHTTPProvider; connection may fail but endpoint parsing runs
	err = tracer.Start(ctx)
	require.NoError(t, err)
}
