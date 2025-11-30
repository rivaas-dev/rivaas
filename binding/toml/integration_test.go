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

package toml_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/binding/toml"
)

// TestIntegration_TOMLBodyBinding tests TOML body binding from HTTP requests
func TestIntegration_TOMLBodyBinding(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	type Config struct {
		Title   string `toml:"title"`
		Name    string `toml:"name"`
		Version string `toml:"version"`
		Port    int    `toml:"port"`
		Debug   bool   `toml:"debug"`
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}

		config, err := toml.TOML[Config](body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		assert.Equal(t, "My Application", config.Title)
		assert.Equal(t, "myapp", config.Name)
		assert.Equal(t, "1.0.0", config.Version)
		assert.Equal(t, 8080, config.Port)
		assert.True(t, config.Debug)
		w.WriteHeader(http.StatusOK)
	})

	body := `
title = "My Application"
name = "myapp"
version = "1.0.0"
port = 8080
debug = true
`
	req := httptest.NewRequest(http.MethodPost, "/config", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/toml")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestIntegration_TOMLNestedStructBinding tests nested TOML struct binding from HTTP requests
func TestIntegration_TOMLNestedStructBinding(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	type Database struct {
		Host     string `toml:"host"`
		Port     int    `toml:"port"`
		Name     string `toml:"name"`
		User     string `toml:"user"`
		Password string `toml:"password"`
	}

	type Server struct {
		Host string `toml:"host"`
		Port int    `toml:"port"`
	}

	type AppConfig struct {
		Title    string   `toml:"title"`
		Server   Server   `toml:"server"`
		Database Database `toml:"database"`
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}

		config, err := toml.TOML[AppConfig](body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		assert.Equal(t, "My Service", config.Title)
		assert.Equal(t, "0.0.0.0", config.Server.Host)
		assert.Equal(t, 8080, config.Server.Port)
		assert.Equal(t, "localhost", config.Database.Host)
		assert.Equal(t, 5432, config.Database.Port)
		assert.Equal(t, "mydb", config.Database.Name)
		w.WriteHeader(http.StatusOK)
	})

	body := `
title = "My Service"

[server]
host = "0.0.0.0"
port = 8080

[database]
host = "localhost"
port = 5432
name = "mydb"
user = "admin"
password = "secret"
`
	req := httptest.NewRequest(http.MethodPost, "/config", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/toml")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestIntegration_TOMLArrayBinding tests TOML array binding from HTTP requests
func TestIntegration_TOMLArrayBinding(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	type ServiceConfig struct {
		Name     string   `toml:"name"`
		Replicas int      `toml:"replicas"`
		Hosts    []string `toml:"hosts"`
		Ports    []int    `toml:"ports"`
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}

		config, err := toml.TOML[ServiceConfig](body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		assert.Equal(t, "api-service", config.Name)
		assert.Equal(t, 3, config.Replicas)
		assert.Equal(t, []string{"host1.example.com", "host2.example.com", "host3.example.com"}, config.Hosts)
		assert.Equal(t, []int{8080, 8081, 8082}, config.Ports)
		w.WriteHeader(http.StatusOK)
	})

	body := `
name = "api-service"
replicas = 3
hosts = ["host1.example.com", "host2.example.com", "host3.example.com"]
ports = [8080, 8081, 8082]
`
	req := httptest.NewRequest(http.MethodPost, "/services", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/toml")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestIntegration_TOMLArrayOfTables tests TOML array of tables binding
func TestIntegration_TOMLArrayOfTables(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	type Product struct {
		Name  string `toml:"name"`
		Price int    `toml:"price"`
	}

	type Catalog struct {
		Title    string    `toml:"title"`
		Products []Product `toml:"products"`
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}

		catalog, err := toml.TOML[Catalog](body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		assert.Equal(t, "Product Catalog", catalog.Title)
		require.Len(t, catalog.Products, 3)
		assert.Equal(t, "Widget", catalog.Products[0].Name)
		assert.Equal(t, 100, catalog.Products[0].Price)
		assert.Equal(t, "Gadget", catalog.Products[1].Name)
		assert.Equal(t, 200, catalog.Products[1].Price)
		w.WriteHeader(http.StatusOK)
	})

	body := `
title = "Product Catalog"

[[products]]
name = "Widget"
price = 100

[[products]]
name = "Gadget"
price = 200

[[products]]
name = "Gizmo"
price = 300
`
	req := httptest.NewRequest(http.MethodPost, "/catalog", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/toml")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestIntegration_TOMLInlineTable tests TOML inline table binding
func TestIntegration_TOMLInlineTable(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	type Point struct {
		X int `toml:"x"`
		Y int `toml:"y"`
	}

	type Config struct {
		Name   string `toml:"name"`
		Origin Point  `toml:"origin"`
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}

		config, err := toml.TOML[Config](body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		assert.Equal(t, "test", config.Name)
		assert.Equal(t, 10, config.Origin.X)
		assert.Equal(t, 20, config.Origin.Y)
		w.WriteHeader(http.StatusOK)
	})

	body := `
name = "test"
origin = { x = 10, y = 20 }
`
	req := httptest.NewRequest(http.MethodPost, "/config", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/toml")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestIntegration_TOMLErrorHandling tests error handling for invalid TOML
func TestIntegration_TOMLErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	tests := []struct {
		name           string
		body           string
		wantStatusCode int
	}{
		{
			name:           "invalid TOML syntax - missing quotes",
			body:           "name = invalid value without quotes",
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name:           "empty body",
			body:           "",
			wantStatusCode: http.StatusOK, // Empty TOML is valid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			type Config struct {
				Name string `toml:"name"`
				Port int    `toml:"port"`
			}

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					http.Error(w, "failed to read body", http.StatusInternalServerError)
					return
				}

				_, err = toml.TOML[Config](body)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodPost, "/config", bytes.NewReader([]byte(tt.body)))
			req.Header.Set("Content-Type", "application/toml")
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)
			assert.Equal(t, tt.wantStatusCode, w.Code)
		})
	}
}

// TestIntegration_TOMLWithMetadata tests TOML binding with metadata for undecoded keys
func TestIntegration_TOMLWithMetadata(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	type Config struct {
		Name string `toml:"name"`
		Port int    `toml:"port"`
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}

		config, meta, err := toml.TOMLWithMetadata[Config](body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		assert.Equal(t, "myapp", config.Name)
		assert.Equal(t, 8080, config.Port)

		// Check for undecoded keys
		undecoded := meta.Undecoded()
		assert.Len(t, undecoded, 1)
		assert.Equal(t, "unknown_field", undecoded[0].String())
		w.WriteHeader(http.StatusOK)
	})

	body := `
name = "myapp"
port = 8080
unknown_field = "should be detected"
`
	req := httptest.NewRequest(http.MethodPost, "/config", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/toml")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestIntegration_TOMLReaderBinding tests TOML binding from io.Reader
func TestIntegration_TOMLReaderBinding(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	type Config struct {
		Name string `toml:"name"`
		Port int    `toml:"port"`
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		config, err := toml.TOMLReader[Config](r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		assert.Equal(t, "myapp", config.Name)
		assert.Equal(t, 8080, config.Port)
		w.WriteHeader(http.StatusOK)
	})

	body := `
name = "myapp"
port = 8080
`
	req := httptest.NewRequest(http.MethodPost, "/config", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/toml")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
