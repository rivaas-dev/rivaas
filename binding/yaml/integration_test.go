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

package yaml_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"rivaas.dev/binding/yaml"
)

// TestIntegration_YAMLBodyBinding tests YAML body binding from HTTP requests
func TestIntegration_YAMLBodyBinding(t *testing.T) {
	t.Parallel()

	type Config struct {
		Name    string `yaml:"name"`
		Version string `yaml:"version"`
		Port    int    `yaml:"port"`
		Debug   bool   `yaml:"debug"`
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}

		config, err := yaml.YAML[Config](body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		assert.Equal(t, "myapp", config.Name)
		assert.Equal(t, "1.0.0", config.Version)
		assert.Equal(t, 8080, config.Port)
		assert.True(t, config.Debug)
		w.WriteHeader(http.StatusOK)
	})

	body := `
name: myapp
version: "1.0.0"
port: 8080
debug: true
`
	req := httptest.NewRequest(http.MethodPost, "/config", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/x-yaml")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestIntegration_YAMLNestedStructBinding tests nested YAML struct binding from HTTP requests
func TestIntegration_YAMLNestedStructBinding(t *testing.T) {
	t.Parallel()

	type Database struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		Name     string `yaml:"name"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
	}

	type Server struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	}

	type AppConfig struct {
		App      string   `yaml:"app"`
		Server   Server   `yaml:"server"`
		Database Database `yaml:"database"`
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}

		config, err := yaml.YAML[AppConfig](body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		assert.Equal(t, "myservice", config.App)
		assert.Equal(t, "0.0.0.0", config.Server.Host)
		assert.Equal(t, 8080, config.Server.Port)
		assert.Equal(t, "localhost", config.Database.Host)
		assert.Equal(t, 5432, config.Database.Port)
		assert.Equal(t, "mydb", config.Database.Name)
		w.WriteHeader(http.StatusOK)
	})

	body := `
app: myservice
server:
  host: 0.0.0.0
  port: 8080
database:
  host: localhost
  port: 5432
  name: mydb
  user: admin
  password: secret
`
	req := httptest.NewRequest(http.MethodPost, "/config", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/x-yaml")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestIntegration_YAMLArrayBinding tests YAML array binding from HTTP requests
func TestIntegration_YAMLArrayBinding(t *testing.T) {
	t.Parallel()

	type ServiceConfig struct {
		Name     string   `yaml:"name"`
		Replicas int      `yaml:"replicas"`
		Hosts    []string `yaml:"hosts"`
		Ports    []int    `yaml:"ports"`
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}

		config, err := yaml.YAML[ServiceConfig](body)
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
name: api-service
replicas: 3
hosts:
  - host1.example.com
  - host2.example.com
  - host3.example.com
ports:
  - 8080
  - 8081
  - 8082
`
	req := httptest.NewRequest(http.MethodPost, "/services", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/x-yaml")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestIntegration_YAMLMapBinding tests YAML map binding from HTTP requests
func TestIntegration_YAMLMapBinding(t *testing.T) {
	t.Parallel()

	type EnvConfig struct {
		Name        string            `yaml:"name"`
		Environment string            `yaml:"environment"`
		Settings    map[string]string `yaml:"settings"`
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}

		config, err := yaml.YAML[EnvConfig](body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		assert.Equal(t, "myapp", config.Name)
		assert.Equal(t, "production", config.Environment)
		assert.Equal(t, "debug", config.Settings["log_level"])
		assert.Equal(t, "us-east-1", config.Settings["region"])
		w.WriteHeader(http.StatusOK)
	})

	body := `
name: myapp
environment: production
settings:
  log_level: debug
  region: us-east-1
  feature_flag: enabled
`
	req := httptest.NewRequest(http.MethodPost, "/env", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/x-yaml")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestIntegration_YAMLErrorHandling tests error handling for invalid YAML
//
//nolint:tparallel // False positive: t.Parallel() is called at both top level and in subtests
func TestIntegration_YAMLErrorHandling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		body           string
		wantStatusCode int
	}{
		{
			name:           "invalid YAML syntax",
			body:           "name: [invalid: unclosed",
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name:           "empty body",
			body:           "",
			wantStatusCode: http.StatusOK, // Empty YAML is valid, results in zero values
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			type Config struct {
				Name string `yaml:"name"`
				Port int    `yaml:"port"`
			}

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					http.Error(w, "failed to read body", http.StatusInternalServerError)
					return
				}

				_, err = yaml.YAML[Config](body)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodPost, "/config", bytes.NewReader([]byte(tt.body)))
			req.Header.Set("Content-Type", "application/x-yaml")
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)
			assert.Equal(t, tt.wantStatusCode, w.Code)
		})
	}
}

// TestIntegration_YAMLWithStrict tests strict mode YAML binding
func TestIntegration_YAMLWithStrict(t *testing.T) {
	t.Parallel()

	type Config struct {
		Name string `yaml:"name"`
		Port int    `yaml:"port"`
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}

		_, err = yaml.YAML[Config](body, yaml.WithStrict())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// YAML with unknown field should fail in strict mode
	body := `
name: myapp
port: 8080
unknown_field: should_error
`
	req := httptest.NewRequest(http.MethodPost, "/config", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/x-yaml")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestIntegration_YAMLReaderBinding tests YAML binding from io.Reader
func TestIntegration_YAMLReaderBinding(t *testing.T) {
	t.Parallel()

	type Config struct {
		Name string `yaml:"name"`
		Port int    `yaml:"port"`
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		config, err := yaml.YAMLReader[Config](r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		assert.Equal(t, "myapp", config.Name)
		assert.Equal(t, 8080, config.Port)
		w.WriteHeader(http.StatusOK)
	})

	body := `
name: myapp
port: 8080
`
	req := httptest.NewRequest(http.MethodPost, "/config", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/x-yaml")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
