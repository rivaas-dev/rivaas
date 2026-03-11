// Copyright 2026 The Rivaas Authors
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
	"crypto/tls"
	"crypto/x509"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithTLS_AndWithMTLS_MutuallyExclusive(t *testing.T) {
	t.Parallel()

	pool := x509.NewCertPool()
	// Use a cert with non-empty Certificate so hasMTLS is true
	serverCert := tls.Certificate{Certificate: [][]byte{[]byte("dummy")}}

	_, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithTLS("server.crt", "server.key"),
		WithMTLS(serverCert, WithClientCAs(pool)),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot use both WithTLS and WithMTLS")
}

func TestWithTLS_RequiresBothCertAndKey(t *testing.T) {
	t.Parallel()

	t.Run("missing cert file", func(t *testing.T) {
		t.Parallel()
		_, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithTLS("", "server.key"),
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "both cert file and key file are required")
	})

	t.Run("missing key file", func(t *testing.T) {
		t.Parallel()
		_, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithTLS("server.crt", ""),
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "both cert file and key file are required")
	})
}

func TestWithTLS_ValidConfig_Succeeds(t *testing.T) {
	t.Parallel()

	a, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithTLS("server.crt", "server.key"),
	)
	require.NoError(t, err)
	require.NotNil(t, a)
	assert.Equal(t, "server.crt", a.config.server.tlsCertFile)
	assert.Equal(t, "server.key", a.config.server.tlsKeyFile)
	assert.Equal(t, DefaultTLSPort, a.config.server.port, "WithTLS alone should set default port to 8443")
}

func TestWithTLS_Only_SetsPortTo8443(t *testing.T) {
	t.Parallel()

	a, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithTLS("a", "b"),
	)
	require.NoError(t, err)
	assert.Equal(t, 8443, a.config.server.port)
}

func TestWithMTLS_Only_SetsPortTo8443(t *testing.T) {
	t.Parallel()

	pool := x509.NewCertPool()
	serverCert := tls.Certificate{Certificate: [][]byte{[]byte("dummy")}}

	a, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithMTLS(serverCert, WithClientCAs(pool)),
	)
	require.NoError(t, err)
	assert.Equal(t, 8443, a.config.server.port)
}

func TestWithPort_BeforeWithTLS_Prevents8443(t *testing.T) {
	t.Parallel()

	a, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithPort(443),
		WithTLS("a", "b"),
	)
	require.NoError(t, err)
	assert.Equal(t, 443, a.config.server.port)
}

func TestWithPort_AfterWithTLS_Overrides(t *testing.T) {
	t.Parallel()

	a, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
		WithTLS("a", "b"),
		WithPort(80),
	)
	require.NoError(t, err)
	assert.Equal(t, 80, a.config.server.port)
}

func TestDefaultConfig_PortRemains8080WithoutTLS(t *testing.T) {
	t.Parallel()

	a, err := New(
		WithServiceName("test"),
		WithServiceVersion("1.0.0"),
	)
	require.NoError(t, err)
	assert.Equal(t, DefaultPort, a.config.server.port)
}
