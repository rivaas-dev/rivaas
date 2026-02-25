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

//go:build integration

package app_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"rivaas.dev/app"
	"rivaas.dev/openapi"
)

var _ = Describe("Server Start", func() {
	Describe("OpenAPI endpoints", func() {
		It("should serve OpenAPI spec when app is started with WithOpenAPI", func() {
			const port = 58100
			a := app.MustNew(
				app.WithServiceName("test"),
				app.WithServiceVersion("1.0.0"),
				app.WithPort(port),
				app.WithServer(app.WithShutdownTimeout(2*time.Second)),
				app.WithOpenAPI(openapi.WithTitle("test-api", "1.0.0")),
			)
			a.GET("/ping", func(c *app.Context) {
				_ = c.String(http.StatusOK, "pong")
			}, app.WithDoc(openapi.WithSummary("Ping")))

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			serverErr := make(chan error, 1)
			go func() {
				serverErr <- a.Start(ctx)
			}()

			// Wait for server to be ready
			time.Sleep(400 * time.Millisecond)

			specURL := fmt.Sprintf("http://127.0.0.1:%d/openapi.json", port)
			resp, err := http.Get(specURL)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(resp.Header.Get("Content-Type")).To(ContainSubstring("application/json"))

			body := make([]byte, 4096)
			n, _ := resp.Body.Read(body)
			Expect(n).To(BeNumerically(">", 0))
			Expect(string(body[:n])).To(ContainSubstring("openapi"))

			cancel()
			select {
			case err := <-serverErr:
				Expect(err).NotTo(HaveOccurred())
			case <-time.After(5 * time.Second):
				Fail("server did not shut down in time")
			}
		})
	})

	Describe("Health endpoints", func() {
		It("should serve /livez and /readyz when app is started with WithHealthEndpoints", func() {
			const port = 58103
			a := app.MustNew(
				app.WithServiceName("test"),
				app.WithServiceVersion("1.0.0"),
				app.WithPort(port),
				app.WithServer(app.WithShutdownTimeout(2*time.Second)),
				app.WithHealthEndpoints(
					app.WithLivenessCheck("ok", func(ctx context.Context) error { return nil }),
					app.WithReadinessCheck("ok", func(ctx context.Context) error { return nil }),
				),
			)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			serverErr := make(chan error, 1)
			go func() {
				serverErr <- a.Start(ctx)
			}()

			time.Sleep(400 * time.Millisecond)

			baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
			resp, err := http.Get(baseURL + "/livez")
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			resp2, err := http.Get(baseURL + "/readyz")
			Expect(err).NotTo(HaveOccurred())
			defer resp2.Body.Close()
			Expect(resp2.StatusCode).To(Equal(http.StatusNoContent))

			cancel()
			select {
			case err := <-serverErr:
				Expect(err).NotTo(HaveOccurred())
			case <-time.After(5 * time.Second):
				Fail("server did not shut down in time")
			}
		})
	})

	Describe("Debug endpoints", func() {
		It("should serve /debug/pprof/ when app is started with WithDebugEndpoints(WithPprof)", func() {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			Expect(err).NotTo(HaveOccurred())
			port := listener.Addr().(*net.TCPAddr).Port
			listener.Close()

			a := app.MustNew(
				app.WithServiceName("test"),
				app.WithServiceVersion("1.0.0"),
				app.WithPort(port),
				app.WithServer(app.WithShutdownTimeout(2*time.Second)),
				app.WithDebugEndpoints(app.WithPprof()),
			)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			serverErr := make(chan error, 1)
			go func() {
				serverErr <- a.Start(ctx)
			}()

			pprofURL := fmt.Sprintf("http://127.0.0.1:%d/debug/pprof/", port)
			var resp *http.Response
			for i := 0; i < 50; i++ {
				time.Sleep(100 * time.Millisecond)
				resp, err = http.Get(pprofURL)
				if err == nil && resp != nil && resp.StatusCode == http.StatusOK {
					break
				}
				if resp != nil {
					resp.Body.Close()
				}
			}
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).NotTo(BeNil())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			cancel()
			select {
			case err := <-serverErr:
				Expect(err).NotTo(HaveOccurred())
			case <-time.After(5 * time.Second):
				Fail("server did not shut down in time")
			}
		})
	})

	Describe("StartTLS", func() {
		It("should serve HTTPS with self-signed certificate", func() {
			const port = 58101
			certPath, keyPath := writeTempSelfSignedCert()
			defer os.Remove(certPath)
			defer os.Remove(keyPath)

			a := app.MustNew(
				app.WithServiceName("test"),
				app.WithServiceVersion("1.0.0"),
				app.WithPort(port),
				app.WithServer(app.WithShutdownTimeout(2*time.Second)),
			)
			a.GET("/", func(c *app.Context) {
				_ = c.String(http.StatusOK, "ok")
			})

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			serverErr := make(chan error, 1)
			go func() {
				serverErr <- a.StartTLS(ctx, certPath, keyPath)
			}()

			time.Sleep(400 * time.Millisecond)

			client := &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				},
			}
			resp, err := client.Get(fmt.Sprintf("https://127.0.0.1:%d/", port))
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			cancel()
			select {
			case err := <-serverErr:
				Expect(err).NotTo(HaveOccurred())
			case <-time.After(5 * time.Second):
				Fail("server did not shut down in time")
			}
		})
	})

	Describe("StartMTLS", func() {
		It("should allow connection when WithAuthorize returns true", func() {
			const port = 58102
			caCert, caKey := generateCA()
			serverCert, serverKey := generateCert(caCert, caKey, "server", nil)
			clientCert, clientKey := generateCert(caCert, caKey, "allowed", nil)

			certPath, keyPath := writeCertKey(serverCert, serverKey)
			defer os.Remove(certPath)
			defer os.Remove(keyPath)

			serverTLS, err := tls.LoadX509KeyPair(certPath, keyPath)
			Expect(err).NotTo(HaveOccurred())

			pool := x509.NewCertPool()
			pool.AddCert(caCert)

			a := app.MustNew(
				app.WithServiceName("test"),
				app.WithServiceVersion("1.0.0"),
				app.WithPort(port),
				app.WithServer(app.WithShutdownTimeout(2*time.Second)),
			)
			a.GET("/", func(c *app.Context) {
				_ = c.String(http.StatusOK, "ok")
			})

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			serverErr := make(chan error, 1)
			go func() {
				serverErr <- a.StartMTLS(ctx, serverTLS,
					app.WithClientCAs(pool),
					app.WithAuthorize(func(cert *x509.Certificate) (string, bool) {
						return cert.Subject.CommonName, cert.Subject.CommonName == "allowed"
					}),
				)
			}()

			time.Sleep(400 * time.Millisecond)

			clientCertPem := pemEncodeCert(clientCert)
			clientKeyPem := pemEncodeKey(clientKey)
			clientTLS, err := tls.X509KeyPair(clientCertPem, clientKeyPem)
			Expect(err).NotTo(HaveOccurred())

			client := &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
						Certificates:       []tls.Certificate{clientTLS},
					},
				},
			}
			resp, err := client.Get(fmt.Sprintf("https://127.0.0.1:%d/", port))
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			cancel()
			select {
			case err := <-serverErr:
				Expect(err).NotTo(HaveOccurred())
			case <-time.After(5 * time.Second):
				Fail("server did not shut down in time")
			}
		})

		It("should close connection when WithAuthorize returns false", func() {
			const port = 58103
			caCert, caKey := generateCA()
			serverCert, serverKey := generateCert(caCert, caKey, "server", nil)
			clientCert, clientKey := generateCert(caCert, caKey, "denied", nil)

			certPath, keyPath := writeCertKey(serverCert, serverKey)
			defer os.Remove(certPath)
			defer os.Remove(keyPath)

			serverTLS, err := tls.LoadX509KeyPair(certPath, keyPath)
			Expect(err).NotTo(HaveOccurred())

			pool := x509.NewCertPool()
			pool.AddCert(caCert)

			a := app.MustNew(
				app.WithServiceName("test"),
				app.WithServiceVersion("1.0.0"),
				app.WithPort(port),
				app.WithServer(app.WithShutdownTimeout(2*time.Second)),
			)
			a.GET("/", func(c *app.Context) {
				_ = c.String(http.StatusOK, "ok")
			})

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			serverErr := make(chan error, 1)
			go func() {
				serverErr <- a.StartMTLS(ctx, serverTLS,
					app.WithClientCAs(pool),
					app.WithAuthorize(func(cert *x509.Certificate) (string, bool) {
						return cert.Subject.CommonName, cert.Subject.CommonName == "allowed"
					}),
				)
			}()

			time.Sleep(400 * time.Millisecond)

			clientCertPem := pemEncodeCert(clientCert)
			clientKeyPem := pemEncodeKey(clientKey)
			clientTLS, err := tls.X509KeyPair(clientCertPem, clientKeyPem)
			Expect(err).NotTo(HaveOccurred())

			client := &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
						Certificates:       []tls.Certificate{clientTLS},
					},
				},
			}
			resp, err := client.Get(fmt.Sprintf("https://127.0.0.1:%d/", port))
			// When WithAuthorize returns false, server closes the connection: we may get an error
			// (connection reset, EOF) or a non-200 response. Either outcome is acceptable.
			if err != nil {
				// Expected: connection closed or reset
				Expect(err.Error()).To(Or(
					ContainSubstring("connection reset"),
					ContainSubstring("EOF"),
					ContainSubstring("refused"),
					ContainSubstring("connection refused"),
				))
			} else {
				defer resp.Body.Close()
				// If we got a response, denied client must not receive 200 OK
				Expect(resp.StatusCode).NotTo(Equal(http.StatusOK))
			}

			cancel()
			select {
			case err := <-serverErr:
				Expect(err).NotTo(HaveOccurred())
			case <-time.After(5 * time.Second):
				Fail("server did not shut down in time")
			}
		})
	})
})

// writeTempSelfSignedCert creates a temporary self-signed cert and key file for TLS tests.
func writeTempSelfSignedCert() (certPath, keyPath string) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	Expect(err).NotTo(HaveOccurred())

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
	Expect(err).NotTo(HaveOccurred())

	certFile, err := os.CreateTemp("", "server-cert-*.pem")
	Expect(err).NotTo(HaveOccurred())
	keyFile, err := os.CreateTemp("", "server-key-*.pem")
	Expect(err).NotTo(HaveOccurred())

	Expect(pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})).To(Succeed())
	keyDER, err := x509.MarshalECPrivateKey(key)
	Expect(err).NotTo(HaveOccurred())
	Expect(pem.Encode(keyFile, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})).To(Succeed())
	Expect(certFile.Close()).To(Succeed())
	Expect(keyFile.Close()).To(Succeed())
	return certFile.Name(), keyFile.Name()
}

func generateCA() (*x509.Certificate, *ecdsa.PrivateKey) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	Expect(err).NotTo(HaveOccurred())
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "ca"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	Expect(err).NotTo(HaveOccurred())
	cert, err := x509.ParseCertificate(certDER)
	Expect(err).NotTo(HaveOccurred())
	return cert, key
}

func generateCert(ca *x509.Certificate, caKey *ecdsa.PrivateKey, commonName string, dnsNames []string) (*x509.Certificate, *ecdsa.PrivateKey) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	Expect(err).NotTo(HaveOccurred())
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(2),
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		DNSNames:              dnsNames,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, ca, &key.PublicKey, caKey)
	Expect(err).NotTo(HaveOccurred())
	cert, err := x509.ParseCertificate(certDER)
	Expect(err).NotTo(HaveOccurred())
	return cert, key
}

func writeCertKey(cert *x509.Certificate, key *ecdsa.PrivateKey) (certPath, keyPath string) {
	certFile, err := os.CreateTemp("", "mtls-cert-*.pem")
	Expect(err).NotTo(HaveOccurred())
	keyFile, err := os.CreateTemp("", "mtls-key-*.pem")
	Expect(err).NotTo(HaveOccurred())
	Expect(pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})).To(Succeed())
	keyDER, err := x509.MarshalECPrivateKey(key)
	Expect(err).NotTo(HaveOccurred())
	Expect(pem.Encode(keyFile, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})).To(Succeed())
	Expect(certFile.Close()).To(Succeed())
	Expect(keyFile.Close()).To(Succeed())
	return certFile.Name(), keyFile.Name()
}

func pemEncodeCert(cert *x509.Certificate) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
}

func pemEncodeKey(key *ecdsa.PrivateKey) []byte {
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		panic(fmt.Sprintf("marshal key: %v", err))
	}
	return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
}
