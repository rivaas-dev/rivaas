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

package app

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
)

// mtlsConfig configures mutual TLS (mTLS) authentication.
// mTLS requires both client and server to present certificates for bidirectional authentication.
type mtlsConfig struct {
	serverCert         tls.Certificate
	clientCAs          *x509.CertPool
	minVersion         uint16
	authorize          func(*x509.Certificate) (principal string, allowed bool)
	getCertificate     func(*tls.ClientHelloInfo) (*tls.Certificate, error)
	getConfigForClient func(*tls.ClientHelloInfo) (*tls.Config, error)
}

// MTLSOption configures mtlsConfig.
type MTLSOption func(*mtlsConfig)

// newMTLSConfig creates a new mTLS configuration with the given server certificate and options.
// The server certificate is required; all other settings are optional.
func newMTLSConfig(serverCert tls.Certificate, opts ...MTLSOption) *mtlsConfig {
	cfg := &mtlsConfig{
		serverCert: serverCert,
		minVersion: tls.VersionTLS13, // Default to TLS 1.3
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return cfg
}

// WithClientCAs sets the certificate authority pool for validating client certificates.
// It is required for mTLS - without it, client certificate validation will fail.
func WithClientCAs(pool *x509.CertPool) MTLSOption {
	return func(cfg *mtlsConfig) {
		cfg.clientCAs = pool
	}
}

// WithMinVersion sets the minimum TLS version to accept.
// Defaults to TLS 1.3 if not specified.
// Use TLS 1.2 or lower only if compatibility is required.
func WithMinVersion(version uint16) MTLSOption {
	return func(cfg *mtlsConfig) {
		cfg.minVersion = version
	}
}

// WithAuthorize sets a callback that maps client certificates to principal identity and authorization.
// The callback returns the principal (e.g., CN, SAN SPIFFE ID) and whether access is allowed.
// If not set, any valid client certificate is accepted.
//
// Example:
//
//	WithAuthorize(func(cert *x509.Certificate) (string, bool) {
//	    // Extract SPIFFE ID from SAN
//	    for _, uri := range cert.URIs {
//	        if uri.Scheme == "spiffe" {
//	            return uri.String(), true
//	        }
//	    }
//	    // Fallback to CN
//	    return cert.Subject.CommonName, cert.Subject.CommonName != ""
//	})
func WithAuthorize(fn func(*x509.Certificate) (principal string, allowed bool)) MTLSOption {
	return func(cfg *mtlsConfig) {
		cfg.authorize = fn
	}
}

// WithSNI sets a callback for SNI (Server Name Indication) support.
// Allows serving different certificates based on the requested hostname.
// If not set, ServerCert is used for all connections.
func WithSNI(fn func(*tls.ClientHelloInfo) (*tls.Certificate, error)) MTLSOption {
	return func(cfg *mtlsConfig) {
		cfg.getCertificate = fn
	}
}

// WithConfigForClient sets a callback for per-client TLS configuration.
// Useful for hot-reloading certificates or dynamic configuration.
// If not set, default configuration is used.
func WithConfigForClient(fn func(*tls.ClientHelloInfo) (*tls.Config, error)) MTLSOption {
	return func(cfg *mtlsConfig) {
		cfg.getConfigForClient = fn
	}
}

// validate validates the mTLS configuration and returns an error if invalid.
func (cfg *mtlsConfig) validate() error {
	if len(cfg.serverCert.Certificate) == 0 {
		return fmt.Errorf("server certificate is required for mTLS")
	}

	if cfg.clientCAs == nil {
		return fmt.Errorf("ClientCAs is required for mTLS (use WithClientCAs option)")
	}

	return nil
}

// buildTLSConfig builds a TLS configuration from mtlsConfig.
func (cfg *mtlsConfig) buildTLSConfig() *tls.Config {
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cfg.serverCert},
		ClientCAs:    cfg.clientCAs,
		ClientAuth:   tls.RequireAndVerifyClientCert, // Require client certificates
		MinVersion:   cfg.minVersion,
	}

	// Note: TLS 1.3 ignores CipherSuites, so we don't set them unless MinVersion < 1.3
	if tlsConfig.MinVersion < tls.VersionTLS13 {
		// Use secure cipher suites for TLS 1.2
		tlsConfig.CipherSuites = []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
		}
	}

	// SNI support
	if cfg.getCertificate != nil {
		tlsConfig.GetCertificate = cfg.getCertificate
	}

	// Per-client configuration (for hot-reload)
	if cfg.getConfigForClient != nil {
		tlsConfig.GetConfigForClient = cfg.getConfigForClient
	}

	return tlsConfig
}

// GetMTLSCertificate extracts the client certificate from an HTTP request.
// It returns the first peer certificate if available, or nil if the request
// is not using mTLS or no certificate is present.
//
// It is useful for extracting principal information (e.g., CN, SAN) in handlers
// after the connection has been authorized via WithAuthorize.
//
// Example:
//
//	func handler(c *router.Context) {
//	    cert := app.GetMTLSCertificate(c.Request)
//	    if cert != nil {
//	        principal := cert.Subject.CommonName
//	        // Use principal for authorization, logging, etc.
//	    }
//	}
func GetMTLSCertificate(req *http.Request) *x509.Certificate {
	if req.TLS == nil || len(req.TLS.PeerCertificates) == 0 {
		return nil
	}

	return req.TLS.PeerCertificates[0]
}
