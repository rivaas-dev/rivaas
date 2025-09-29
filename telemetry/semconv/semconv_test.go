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

package semconv

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServiceMetadataConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		constant string
		want     string
	}{
		{
			name:     "ServiceName",
			constant: ServiceName,
			want:     "service.name",
		},
		{
			name:     "ServiceVersion",
			constant: ServiceVersion,
			want:     "service.version",
		},
		{
			name:     "ServiceNamespace",
			constant: ServiceNamespace,
			want:     "service.namespace",
		},
		{
			name:     "DeploymentEnviron",
			constant: DeploymentEnviron,
			want:     "deployment.environment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.NotEmpty(t, tt.constant, "constant should not be empty")
			assert.Equal(t, tt.want, tt.constant, "constant should match expected value")
		})
	}
}

func TestHTTPAttributeConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		constant string
		want     string
	}{
		{
			name:     "HTTPMethod",
			constant: HTTPMethod,
			want:     "http.method",
		},
		{
			name:     "HTTPRoute",
			constant: HTTPRoute,
			want:     "http.route",
		},
		{
			name:     "HTTPTarget",
			constant: HTTPTarget,
			want:     "http.target",
		},
		{
			name:     "HTTPStatusCode",
			constant: HTTPStatusCode,
			want:     "http.status_code",
		},
		{
			name:     "HTTPScheme",
			constant: HTTPScheme,
			want:     "http.scheme",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.NotEmpty(t, tt.constant, "constant should not be empty")
			assert.Equal(t, tt.want, tt.constant, "constant should match expected value")
		})
	}
}

func TestNetworkAttributeConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		constant string
		want     string
	}{
		{
			name:     "NetworkPeerIP",
			constant: NetworkPeerIP,
			want:     "network.peer.ip",
		},
		{
			name:     "NetworkClientIP",
			constant: NetworkClientIP,
			want:     "network.client.ip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.NotEmpty(t, tt.constant, "constant should not be empty")
			assert.Equal(t, tt.want, tt.constant, "constant should match expected value")
		})
	}
}

func TestTraceCorrelationConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		constant string
		want     string
	}{
		{
			name:     "TraceID",
			constant: TraceID,
			want:     "trace_id",
		},
		{
			name:     "SpanID",
			constant: SpanID,
			want:     "span_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.NotEmpty(t, tt.constant, "constant should not be empty")
			assert.Equal(t, tt.want, tt.constant, "constant should match expected value")
		})
	}
}

func TestRequestAttributeConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		constant string
		want     string
	}{
		{
			name:     "RequestID",
			constant: RequestID,
			want:     "req.id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.NotEmpty(t, tt.constant, "constant should not be empty")
			assert.Equal(t, tt.want, tt.constant, "constant should match expected value")
		})
	}
}

func TestConstantsUniqueness(t *testing.T) {
	t.Parallel()

	allConstants := []string{
		ServiceName,
		ServiceVersion,
		ServiceNamespace,
		DeploymentEnviron,
		HTTPMethod,
		HTTPRoute,
		HTTPTarget,
		HTTPStatusCode,
		HTTPScheme,
		NetworkPeerIP,
		NetworkClientIP,
		TraceID,
		SpanID,
		RequestID,
	}

	seen := make(map[string]bool)
	for _, constant := range allConstants {
		assert.False(t, seen[constant], "constant %q should be unique", constant)
		seen[constant] = true
	}
}
