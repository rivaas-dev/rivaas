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

package msgpack_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mp "github.com/vmihailenco/msgpack/v5"

	"rivaas.dev/binding/msgpack"
)

// TestIntegration_MsgPackBodyBinding tests MessagePack body binding from HTTP requests
func TestIntegration_MsgPackBodyBinding(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	type Config struct {
		Name    string `msgpack:"name"`
		Version string `msgpack:"version"`
		Port    int    `msgpack:"port"`
		Debug   bool   `msgpack:"debug"`
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}

		config, err := msgpack.MsgPack[Config](body)
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

	// Create MessagePack encoded body
	input := Config{
		Name:    "myapp",
		Version: "1.0.0",
		Port:    8080,
		Debug:   true,
	}
	body, err := mp.Marshal(&input)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/msgpack")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestIntegration_MsgPackNestedStructBinding tests nested MessagePack struct binding
func TestIntegration_MsgPackNestedStructBinding(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	type Database struct {
		Host     string `msgpack:"host"`
		Port     int    `msgpack:"port"`
		Name     string `msgpack:"name"`
		User     string `msgpack:"user"`
		Password string `msgpack:"password"`
	}

	type Server struct {
		Host string `msgpack:"host"`
		Port int    `msgpack:"port"`
	}

	type AppConfig struct {
		App      string   `msgpack:"app"`
		Server   Server   `msgpack:"server"`
		Database Database `msgpack:"database"`
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}

		config, err := msgpack.MsgPack[AppConfig](body)
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

	// Create MessagePack encoded body
	input := AppConfig{
		App: "myservice",
		Server: Server{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Database: Database{
			Host:     "localhost",
			Port:     5432,
			Name:     "mydb",
			User:     "admin",
			Password: "secret",
		},
	}
	body, err := mp.Marshal(&input)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/msgpack")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestIntegration_MsgPackArrayBinding tests MessagePack array binding
func TestIntegration_MsgPackArrayBinding(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	type ServiceConfig struct {
		Name     string   `msgpack:"name"`
		Replicas int      `msgpack:"replicas"`
		Hosts    []string `msgpack:"hosts"`
		Ports    []int    `msgpack:"ports"`
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}

		config, err := msgpack.MsgPack[ServiceConfig](body)
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

	// Create MessagePack encoded body
	input := ServiceConfig{
		Name:     "api-service",
		Replicas: 3,
		Hosts:    []string{"host1.example.com", "host2.example.com", "host3.example.com"},
		Ports:    []int{8080, 8081, 8082},
	}
	body, err := mp.Marshal(&input)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/services", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/msgpack")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestIntegration_MsgPackMapBinding tests MessagePack map binding
func TestIntegration_MsgPackMapBinding(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	type EnvConfig struct {
		Name        string            `msgpack:"name"`
		Environment string            `msgpack:"environment"`
		Settings    map[string]string `msgpack:"settings"`
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}

		config, err := msgpack.MsgPack[EnvConfig](body)
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

	// Create MessagePack encoded body
	input := EnvConfig{
		Name:        "myapp",
		Environment: "production",
		Settings: map[string]string{
			"log_level":    "debug",
			"region":       "us-east-1",
			"feature_flag": "enabled",
		},
	}
	body, err := mp.Marshal(&input)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/env", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/msgpack")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestIntegration_MsgPackErrorHandling tests error handling for invalid MessagePack
func TestIntegration_MsgPackErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	type Config struct {
		Name string `msgpack:"name"`
		Port int    `msgpack:"port"`
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}

		_, err = msgpack.MsgPack[Config](body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// Invalid MessagePack data
	req := httptest.NewRequest(http.MethodPost, "/config", bytes.NewReader([]byte("invalid msgpack data")))
	req.Header.Set("Content-Type", "application/msgpack")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestIntegration_MsgPackWithJSONTag tests MessagePack binding with JSON tags
func TestIntegration_MsgPackWithJSONTag(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	type Config struct {
		Name string `json:"name"`
		Port int    `json:"port"`
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}

		config, err := msgpack.MsgPack[Config](body, msgpack.WithJSONTag())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		assert.Equal(t, "myapp", config.Name)
		assert.Equal(t, 8080, config.Port)
		w.WriteHeader(http.StatusOK)
	})

	// Create MessagePack encoded body using JSON tags
	input := Config{
		Name: "myapp",
		Port: 8080,
	}
	buf := &bytes.Buffer{}
	enc := mp.NewEncoder(buf)
	enc.SetCustomStructTag("json")
	err := enc.Encode(&input)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/config", bytes.NewReader(buf.Bytes()))
	req.Header.Set("Content-Type", "application/msgpack")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestIntegration_MsgPackWithDisallowUnknown tests MessagePack binding with unknown field rejection
func TestIntegration_MsgPackWithDisallowUnknown(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	type Source struct {
		Name    string `msgpack:"name"`
		Port    int    `msgpack:"port"`
		Unknown string `msgpack:"unknown_field"`
	}

	type Target struct {
		Name string `msgpack:"name"`
		Port int    `msgpack:"port"`
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}

		_, err = msgpack.MsgPack[Target](body, msgpack.WithDisallowUnknown())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// Create MessagePack encoded body with extra field
	input := Source{
		Name:    "myapp",
		Port:    8080,
		Unknown: "extra",
	}
	body, err := mp.Marshal(&input)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/msgpack")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestIntegration_MsgPackReaderBinding tests MessagePack binding from io.Reader
func TestIntegration_MsgPackReaderBinding(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	type Config struct {
		Name string `msgpack:"name"`
		Port int    `msgpack:"port"`
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		config, err := msgpack.MsgPackReader[Config](r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		assert.Equal(t, "myapp", config.Name)
		assert.Equal(t, 8080, config.Port)
		w.WriteHeader(http.StatusOK)
	})

	// Create MessagePack encoded body
	input := Config{
		Name: "myapp",
		Port: 8080,
	}
	body, err := mp.Marshal(&input)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/msgpack")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestIntegration_MsgPackLargePayload tests MessagePack binding with larger payloads
func TestIntegration_MsgPackLargePayload(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	type Item struct {
		ID    int    `msgpack:"id"`
		Name  string `msgpack:"name"`
		Value int    `msgpack:"value"`
	}

	type Payload struct {
		Items []Item `msgpack:"items"`
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}

		payload, err := msgpack.MsgPack[Payload](body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		assert.Len(t, payload.Items, 100)
		assert.Equal(t, 0, payload.Items[0].ID)
		assert.Equal(t, 99, payload.Items[99].ID)
		w.WriteHeader(http.StatusOK)
	})

	// Create MessagePack encoded body with 100 items
	input := Payload{
		Items: make([]Item, 100),
	}
	for i := range 100 {
		input.Items[i] = Item{
			ID:    i,
			Name:  "item",
			Value: i * 10,
		}
	}
	body, err := mp.Marshal(&input)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/items", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/msgpack")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
