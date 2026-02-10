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

package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTracingConfig_options(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cfg      TracingConfig
		wantErr  bool
		contains string
	}{
		{
			name:    "stdout provider",
			cfg:     TracingConfig{Provider: TracingStdout},
			wantErr: false,
		},
		{
			name:    "otlp provider with default endpoint",
			cfg:     TracingConfig{Provider: TracingOTLP},
			wantErr: false,
		},
		{
			name:    "otlp provider with custom endpoint and insecure",
			cfg:     TracingConfig{Provider: TracingOTLP, Endpoint: "host:4317", Insecure: true},
			wantErr: false,
		},
		{
			name:    "otlp-http provider with default endpoint",
			cfg:     TracingConfig{Provider: TracingOTLPHTTP},
			wantErr: false,
		},
		{
			name:    "noop provider",
			cfg:     TracingConfig{Provider: TracingNoop},
			wantErr: false,
		},
		{
			name:    "empty provider defaults to noop",
			cfg:     TracingConfig{Provider: ""},
			wantErr: false,
		},
		{
			name:    "sample rate in range adds option",
			cfg:     TracingConfig{Provider: TracingNoop, SampleRate: 0.5},
			wantErr: false,
		},
		{
			name:     "unknown provider returns error",
			cfg:      TracingConfig{Provider: TracingProvider("invalid")},
			wantErr:  true,
			contains: "unknown provider",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			opts, err := tt.cfg.options()
			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.contains)
				return
			}
			require.NoError(t, err)
			assert.NotEmpty(t, opts)
		})
	}
}

func TestMetricsConfig_options(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cfg      MetricsConfig
		wantErr  bool
		contains string
	}{
		{
			name:    "prometheus provider with defaults",
			cfg:     MetricsConfig{Provider: MetricsPrometheus},
			wantErr: false,
		},
		{
			name:    "empty provider defaults to prometheus",
			cfg:     MetricsConfig{Provider: ""},
			wantErr: false,
		},
		{
			name:    "prometheus with custom endpoint and path",
			cfg:     MetricsConfig{Provider: MetricsPrometheus, Endpoint: ":9091", Path: "/custom"},
			wantErr: false,
		},
		{
			name:    "otlp provider with default endpoint",
			cfg:     MetricsConfig{Provider: MetricsOTLP},
			wantErr: false,
		},
		{
			name:    "stdout provider",
			cfg:     MetricsConfig{Provider: MetricsStdout},
			wantErr: false,
		},
		{
			name:     "unknown provider returns error",
			cfg:      MetricsConfig{Provider: MetricsProvider("invalid")},
			wantErr:  true,
			contains: "unknown provider",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			opts, err := tt.cfg.options()
			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.contains)
				return
			}
			require.NoError(t, err)
			assert.NotEmpty(t, opts)
		})
	}
}

func TestLoggingConfig_options(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cfg      LoggingConfig
		wantErr  bool
		contains string
	}{
		{
			name:    "console handler with info level",
			cfg:     LoggingConfig{Handler: LoggingConsole, Level: LoggingInfo},
			wantErr: false,
		},
		{
			name:    "empty handler defaults to console",
			cfg:     LoggingConfig{Handler: "", Level: LoggingInfo},
			wantErr: false,
		},
		{
			name:    "json handler",
			cfg:     LoggingConfig{Handler: LoggingJSON, Level: LoggingInfo},
			wantErr: false,
		},
		{
			name:    "debug level",
			cfg:     LoggingConfig{Handler: LoggingConsole, Level: LoggingDebug},
			wantErr: false,
		},
		{
			name:    "warn level",
			cfg:     LoggingConfig{Handler: LoggingConsole, Level: LoggingWarn},
			wantErr: false,
		},
		{
			name:    "error level",
			cfg:     LoggingConfig{Handler: LoggingConsole, Level: LoggingError},
			wantErr: false,
		},
		{
			name:    "empty level defaults to info",
			cfg:     LoggingConfig{Handler: LoggingConsole, Level: ""},
			wantErr: false,
		},
		{
			name:     "unknown handler returns error",
			cfg:      LoggingConfig{Handler: LoggingHandler("invalid"), Level: LoggingInfo},
			wantErr:  true,
			contains: "unknown handler",
		},
		{
			name:     "unknown level returns error",
			cfg:      LoggingConfig{Handler: LoggingConsole, Level: LoggingLevel("invalid")},
			wantErr:  true,
			contains: "unknown level",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			opts, err := tt.cfg.options()
			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.contains)
				return
			}
			require.NoError(t, err)
			assert.NotEmpty(t, opts)
		})
	}
}

func TestWithObservabilityFromConfig_validConfig(t *testing.T) {
	t.Parallel()

	cfg := ObservabilityConfig{
		Tracing: TracingConfig{Provider: TracingNoop},
		Metrics: MetricsConfig{Provider: MetricsPrometheus},
		Logging: LoggingConfig{Handler: LoggingConsole, Level: LoggingInfo},
	}
	app, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithObservabilityFromConfig(cfg),
	)
	require.NoError(t, err)
	require.NotNil(t, app)
	assert.NotNil(t, app.logging)
	assert.NotNil(t, app.metrics)
	assert.NotNil(t, app.tracing)
}

func TestWithObservabilityFromConfig_excludePathsApplied(t *testing.T) {
	t.Parallel()

	cfg := ObservabilityConfig{
		Tracing:         TracingConfig{Provider: TracingNoop},
		ExcludePaths:    []string{"/healthz", "/readyz"},
		ExcludePrefixes: []string{"/admin/"},
	}
	app, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithObservabilityFromConfig(cfg),
	)
	require.NoError(t, err)
	require.NotNil(t, app)
	assert.True(t, app.config.observability.pathFilter.shouldExclude("/healthz"))
	assert.True(t, app.config.observability.pathFilter.shouldExclude("/readyz"))
	assert.True(t, app.config.observability.pathFilter.shouldExclude("/admin/foo"))
}

func TestWithObservabilityFromConfig_invalidTracingProviderPanics(t *testing.T) {
	t.Parallel()

	cfg := ObservabilityConfig{
		Tracing: TracingConfig{Provider: TracingProvider("invalid")},
	}
	assert.Panics(t, func() {
		MustNew(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithObservabilityFromConfig(cfg),
		)
	})
}
