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

package proto_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/binding/proto"
	"rivaas.dev/binding/proto/testdata"

	goproto "google.golang.org/protobuf/proto"
)

// TestIntegration_ProtoBodyBinding tests Protocol Buffers body binding from HTTP requests
func TestIntegration_ProtoBodyBinding(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}

		user, err := proto.Proto[*testdata.User](body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		assert.Equal(t, "John", user.GetName())
		assert.Equal(t, "john@example.com", user.GetEmail())
		assert.Equal(t, int32(30), user.GetAge())
		assert.True(t, user.GetActive())
		w.WriteHeader(http.StatusOK)
	})

	// Create Protocol Buffers encoded body
	user := &testdata.User{
		Name:   "John",
		Email:  "john@example.com",
		Age:    30,
		Active: true,
	}
	body, err := goproto.Marshal(user)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/user", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/x-protobuf")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestIntegration_ProtoNestedStructBinding tests nested Protocol Buffers binding
func TestIntegration_ProtoNestedStructBinding(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}

		config, err := proto.Proto[*testdata.Config](body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		assert.Equal(t, "My Service", config.GetTitle())
		assert.Equal(t, "0.0.0.0", config.GetServer().GetHost())
		assert.Equal(t, int32(8080), config.GetServer().GetPort())
		assert.Equal(t, "localhost", config.GetDatabase().GetHost())
		assert.Equal(t, int32(5432), config.GetDatabase().GetPort())
		assert.Equal(t, "mydb", config.GetDatabase().GetName())
		w.WriteHeader(http.StatusOK)
	})

	// Create Protocol Buffers encoded body
	config := &testdata.Config{
		Title: "My Service",
		Server: &testdata.Server{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Database: &testdata.Database{
			Host:     "localhost",
			Port:     5432,
			Name:     "mydb",
			User:     "admin",
			Password: "secret",
		},
	}
	body, err := goproto.Marshal(config)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/x-protobuf")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestIntegration_ProtoRepeatedFieldsBinding tests Protocol Buffers repeated field binding
func TestIntegration_ProtoRepeatedFieldsBinding(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}

		product, err := proto.Proto[*testdata.Product](body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		assert.Equal(t, "Widget", product.GetName())
		assert.Equal(t, []string{"electronics", "gadget", "sale"}, product.GetTags())
		assert.Equal(t, []int32{100, 200, 300}, product.GetPrices())
		w.WriteHeader(http.StatusOK)
	})

	// Create Protocol Buffers encoded body
	product := &testdata.Product{
		Name:   "Widget",
		Tags:   []string{"electronics", "gadget", "sale"},
		Prices: []int32{100, 200, 300},
	}
	body, err := goproto.Marshal(product)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/product", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/x-protobuf")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestIntegration_ProtoMapFieldsBinding tests Protocol Buffers map field binding
func TestIntegration_ProtoMapFieldsBinding(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}

		settings, err := proto.Proto[*testdata.Settings](body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		assert.Equal(t, "MySettings", settings.GetName())
		assert.Equal(t, "value1", settings.GetMetadata()["key1"])
		assert.Equal(t, "value2", settings.GetMetadata()["key2"])
		w.WriteHeader(http.StatusOK)
	})

	// Create Protocol Buffers encoded body
	settings := &testdata.Settings{
		Name: "MySettings",
		Metadata: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}
	body, err := goproto.Marshal(settings)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/x-protobuf")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestIntegration_ProtoErrorHandling tests error handling for invalid Protocol Buffers
func TestIntegration_ProtoErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}

		_, err = proto.Proto[*testdata.User](body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// Invalid Protocol Buffers data
	req := httptest.NewRequest(http.MethodPost, "/user", bytes.NewReader([]byte("invalid proto data")))
	req.Header.Set("Content-Type", "application/x-protobuf")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestIntegration_ProtoReaderBinding tests Protocol Buffers binding from io.Reader
func TestIntegration_ProtoReaderBinding(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := proto.ProtoReader[*testdata.User](r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		assert.Equal(t, "Alice", user.GetName())
		assert.Equal(t, "alice@example.com", user.GetEmail())
		w.WriteHeader(http.StatusOK)
	})

	// Create Protocol Buffers encoded body
	user := &testdata.User{
		Name:  "Alice",
		Email: "alice@example.com",
		Age:   25,
	}
	body, err := goproto.Marshal(user)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/user", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/x-protobuf")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestIntegration_ProtoWithDiscardUnknown tests Protocol Buffers binding with unknown field handling
func TestIntegration_ProtoWithDiscardUnknown(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}

		user, err := proto.Proto[*testdata.User](body, proto.WithDiscardUnknown())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		assert.Equal(t, "Bob", user.GetName())
		w.WriteHeader(http.StatusOK)
	})

	// Create Protocol Buffers encoded body
	user := &testdata.User{
		Name:   "Bob",
		Email:  "bob@example.com",
		Age:    35,
		Active: false,
	}
	body, err := goproto.Marshal(user)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/user", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/x-protobuf")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestIntegration_ProtoWithValidator tests Protocol Buffers binding with validation
func TestIntegration_ProtoWithValidator(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	validator := &ageValidator{minAge: 18}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}

		user, err := proto.Proto[*testdata.User](body, proto.WithValidator(validator))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		assert.Equal(t, "Adult", user.GetName())
		w.WriteHeader(http.StatusOK)
	})

	// Create Protocol Buffers encoded body with valid age
	user := &testdata.User{
		Name:  "Adult",
		Email: "adult@example.com",
		Age:   25,
	}
	body, err := goproto.Marshal(user)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/user", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/x-protobuf")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestIntegration_ProtoLargePayload tests Protocol Buffers binding with larger payloads
func TestIntegration_ProtoLargePayload(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}

		product, err := proto.Proto[*testdata.Product](body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		assert.Len(t, product.GetTags(), 100)
		assert.Len(t, product.GetPrices(), 100)
		w.WriteHeader(http.StatusOK)
	})

	// Create Protocol Buffers encoded body with 100 items
	tags := make([]string, 100)
	prices := make([]int32, 100)
	for i := range 100 {
		tags[i] = "tag"
		prices[i] = int32(i * 10)
	}

	product := &testdata.Product{
		Name:   "Large Product",
		Tags:   tags,
		Prices: prices,
	}
	body, err := goproto.Marshal(product)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/product", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/x-protobuf")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestIntegration_ProtoEmptyBody tests Protocol Buffers binding with empty body
func TestIntegration_ProtoEmptyBody(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}

		user, err := proto.Proto[*testdata.User](body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Empty proto results in zero values
		assert.Empty(t, user.GetName())
		assert.Empty(t, user.GetEmail())
		assert.Equal(t, int32(0), user.GetAge())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/user", bytes.NewReader([]byte{}))
	req.Header.Set("Content-Type", "application/x-protobuf")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// ageValidator is a simple validator for testing.
type ageValidator struct {
	minAge int32
}

func (v *ageValidator) Validate(data any) error {
	// For this test, we just return nil (always valid)
	// In real code, you'd cast and validate
	return nil
}
