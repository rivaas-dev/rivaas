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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errNoTLSConfig is a sentinel for GetConfigForClient returning no custom config (nilnil).
var errNoTLSConfig = errors.New("no TLS config")

// mustGenServerCert returns a minimal tls.Certificate and x509.CertPool for unit tests.
func mustGenServerCert(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err)
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	tlsCert := tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  key,
	}
	return tlsCert, pool
}

func TestNewMTLSConfig_defaults(t *testing.T) {
	t.Parallel()

	serverCert, pool := mustGenServerCert(t)
	cfg := newMTLSConfig(serverCert, WithClientCAs(pool))
	require.NotNil(t, cfg)
	assert.EqualValues(t, tls.VersionTLS13, cfg.minVersion)
	assert.NotNil(t, cfg.clientCAs)
	assert.Nil(t, cfg.authorize)
	assert.Nil(t, cfg.getCertificate)
	assert.Nil(t, cfg.getConfigForClient)
}

func TestNewMTLSConfig_withMinVersion(t *testing.T) {
	t.Parallel()

	serverCert, pool := mustGenServerCert(t)
	cfg := newMTLSConfig(serverCert, WithClientCAs(pool), WithMinVersion(tls.VersionTLS12))
	require.NotNil(t, cfg)
	assert.EqualValues(t, tls.VersionTLS12, cfg.minVersion)
}

func TestNewMTLSConfig_withMinVersionClampsToTLS12(t *testing.T) {
	t.Parallel()

	serverCert, pool := mustGenServerCert(t)
	cfg := newMTLSConfig(serverCert, WithClientCAs(pool), WithMinVersion(0))
	require.NotNil(t, cfg)
	assert.EqualValues(t, tls.VersionTLS12, cfg.minVersion)
}

func TestNewMTLSConfig_withAuthorize(t *testing.T) {
	t.Parallel()

	serverCert, pool := mustGenServerCert(t)
	called := false
	authorize := func(*x509.Certificate) (string, bool) {
		called = true
		return "principal", true
	}
	cfg := newMTLSConfig(serverCert, WithClientCAs(pool), WithAuthorize(authorize))
	require.NotNil(t, cfg)
	require.NotNil(t, cfg.authorize)
	principal, allowed := cfg.authorize(nil)
	assert.True(t, called)
	assert.Equal(t, "principal", principal)
	assert.True(t, allowed)
}

func TestNewMTLSConfig_withSNI(t *testing.T) {
	t.Parallel()

	serverCert, pool := mustGenServerCert(t)
	called := false
	sniFn := func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
		called = true
		return &serverCert, nil
	}
	cfg := newMTLSConfig(serverCert, WithClientCAs(pool), WithSNI(sniFn))
	require.NotNil(t, cfg)
	require.NotNil(t, cfg.getCertificate)
	cert, err := cfg.getCertificate(nil)
	require.NoError(t, err)
	assert.True(t, called)
	assert.NotNil(t, cert)
}

func TestNewMTLSConfig_withConfigForClient(t *testing.T) {
	t.Parallel()

	serverCert, pool := mustGenServerCert(t)
	called := false
	configFn := func(*tls.ClientHelloInfo) (*tls.Config, error) {
		called = true
		return nil, errNoTLSConfig
	}
	cfg := newMTLSConfig(serverCert, WithClientCAs(pool), WithConfigForClient(configFn))
	require.NotNil(t, cfg)
	require.NotNil(t, cfg.getConfigForClient)
	_, err := cfg.getConfigForClient(nil)
	require.ErrorIs(t, err, errNoTLSConfig)
	assert.True(t, called)
}

func TestMTLSConfig_validate(t *testing.T) {
	t.Parallel()

	serverCert, pool := mustGenServerCert(t)

	tests := []struct {
		name     string
		cfg      *mtlsConfig
		wantErr  bool
		contains string
	}{
		{
			name:     "empty server certificate returns error",
			cfg:      &mtlsConfig{clientCAs: pool},
			wantErr:  true,
			contains: "server certificate is required",
		},
		{
			name:     "nil ClientCAs returns error",
			cfg:      &mtlsConfig{serverCert: serverCert},
			wantErr:  true,
			contains: "ClientCAs is required",
		},
		{
			name:    "valid config returns nil",
			cfg:     newMTLSConfig(serverCert, WithClientCAs(pool)),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cfg.validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.contains)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestMTLSConfig_buildTLSConfig(t *testing.T) {
	t.Parallel()

	serverCert, pool := mustGenServerCert(t)
	cfg := newMTLSConfig(serverCert, WithClientCAs(pool))
	tlsCfg := cfg.buildTLSConfig()
	require.NotNil(t, tlsCfg)
	assert.Equal(t, tls.RequireAndVerifyClientCert, tlsCfg.ClientAuth)
	assert.EqualValues(t, tls.VersionTLS13, tlsCfg.MinVersion)
	assert.Len(t, tlsCfg.Certificates, 1)
	assert.Equal(t, cfg.clientCAs, tlsCfg.ClientCAs)
	assert.Nil(t, tlsCfg.GetCertificate)
	assert.Nil(t, tlsCfg.GetConfigForClient)
}

func TestMTLSConfig_buildTLSConfig_TLS12SetsCipherSuites(t *testing.T) {
	t.Parallel()

	serverCert, pool := mustGenServerCert(t)
	cfg := newMTLSConfig(serverCert, WithClientCAs(pool), WithMinVersion(tls.VersionTLS12))
	tlsCfg := cfg.buildTLSConfig()
	require.NotNil(t, tlsCfg)
	assert.EqualValues(t, tls.VersionTLS12, tlsCfg.MinVersion)
	assert.NotEmpty(t, tlsCfg.CipherSuites)
}

func TestMTLSConfig_buildTLSConfig_withGetCertificate(t *testing.T) {
	t.Parallel()

	serverCert, pool := mustGenServerCert(t)
	sniFn := func(*tls.ClientHelloInfo) (*tls.Certificate, error) { return &serverCert, nil }
	cfg := newMTLSConfig(serverCert, WithClientCAs(pool), WithSNI(sniFn))
	tlsCfg := cfg.buildTLSConfig()
	require.NotNil(t, tlsCfg)
	assert.NotNil(t, tlsCfg.GetCertificate)
}

func TestMTLSConfig_buildTLSConfig_withGetConfigForClient(t *testing.T) {
	t.Parallel()

	serverCert, pool := mustGenServerCert(t)
	configFn := func(*tls.ClientHelloInfo) (*tls.Config, error) { return nil, errNoTLSConfig }
	cfg := newMTLSConfig(serverCert, WithClientCAs(pool), WithConfigForClient(configFn))
	tlsCfg := cfg.buildTLSConfig()
	require.NotNil(t, tlsCfg)
	assert.NotNil(t, tlsCfg.GetConfigForClient)
}

func TestGetMTLSCertificate(t *testing.T) {
	t.Parallel()

	_, pool := mustGenServerCert(t)
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "client"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	peerCert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err)
	_ = pool

	tests := []struct {
		name string
		req  *http.Request
		want *x509.Certificate
	}{
		{
			name: "nil TLS returns nil",
			req:  httptest.NewRequest(http.MethodGet, "/", nil),
			want: nil,
		},
		{
			name: "empty PeerCertificates returns nil",
			req:  &http.Request{TLS: &tls.ConnectionState{PeerCertificates: nil}},
			want: nil,
		},
		{
			name: "one peer cert returns first cert",
			req:  &http.Request{TLS: &tls.ConnectionState{PeerCertificates: []*x509.Certificate{peerCert}}},
			want: peerCert,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := GetMTLSCertificate(tt.req)
			assert.Equal(t, tt.want, got)
		})
	}
}
