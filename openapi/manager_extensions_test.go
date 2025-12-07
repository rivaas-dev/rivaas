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

package openapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager_Extensions_Integration(t *testing.T) {
	t.Parallel()

	t.Run("root-level extensions", func(t *testing.T) {
		t.Parallel()
		cfg := MustNew(
			WithTitle("Test API", "1.0.0"),
			WithExtension("x-api-id", "api-123"),
			WithExtension("x-version", 2),
		)

		mgr := NewManager(cfg)
		mgr.Register(http.MethodGet, "/users/:id").
			Doc("Get user", "Retrieves a user by ID").
			Response(200, map[string]string{"id": "string"})

		specJSON, _, err := mgr.GenerateSpec()
		require.NoError(t, err)

		var spec map[string]any
		require.NoError(t, json.Unmarshal(specJSON, &spec))

		// Verify extensions are in the root spec
		assert.Equal(t, "api-123", spec["x-api-id"])
		assert.Equal(t, float64(2), spec["x-version"]) //nolint:testifylint // exact integer comparison
	})

	t.Run("info extensions", func(t *testing.T) {
		t.Parallel()
		cfg := MustNew(
			WithTitle("Test API", "1.0.0"),
			WithInfoExtension("x-api-category", "public"),
		)

		mgr := NewManager(cfg)
		mgr.Register(http.MethodGet, "/test").Response(200, map[string]string{"id": "string"})
		specJSON, _, err := mgr.GenerateSpec()
		require.NoError(t, err)

		var spec map[string]any
		require.NoError(t, json.Unmarshal(specJSON, &spec))

		info, ok := spec["info"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "public", info["x-api-category"])
	})

	t.Run("server extensions", func(t *testing.T) {
		t.Parallel()
		cfg := MustNew(
			WithTitle("Test API", "1.0.0"),
			WithServer("https://api.example.com", "Production"),
		)
		cfg.Servers[0].Extensions = map[string]any{
			"x-region": "us-east-1",
		}

		mgr := NewManager(cfg)
		mgr.Register(http.MethodGet, "/test").Response(200, map[string]string{"id": "string"})
		specJSON, _, err := mgr.GenerateSpec()
		require.NoError(t, err)

		var spec map[string]any
		require.NoError(t, json.Unmarshal(specJSON, &spec))

		servers, ok := spec["servers"].([]any)
		require.True(t, ok)
		require.Len(t, servers, 1)

		server, ok := servers[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "us-east-1", server["x-region"])
	})

	t.Run("tag extensions", func(t *testing.T) {
		t.Parallel()
		cfg := MustNew(
			WithTitle("Test API", "1.0.0"),
			WithTag("users", "User operations"),
		)
		cfg.Tags[0].Extensions = map[string]any{
			"x-tag-color": "blue",
		}

		mgr := NewManager(cfg)
		mgr.Register(http.MethodGet, "/test").Response(200, map[string]string{"id": "string"})
		specJSON, _, err := mgr.GenerateSpec()
		require.NoError(t, err)

		var spec map[string]any
		require.NoError(t, json.Unmarshal(specJSON, &spec))

		tags, ok := spec["tags"].([]any)
		require.True(t, ok)
		require.Len(t, tags, 1)

		tag, ok := tags[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "blue", tag["x-tag-color"])
	})

	t.Run("contact extensions", func(t *testing.T) {
		t.Parallel()
		cfg := MustNew(
			WithTitle("Test API", "1.0.0"),
			WithContact("Support", "https://example.com", "support@example.com"),
		)
		cfg.Info.Contact.Extensions = map[string]any{
			"x-contact-id": "contact-123",
		}

		mgr := NewManager(cfg)
		mgr.Register(http.MethodGet, "/test").Response(200, map[string]string{"id": "string"})
		specJSON, _, err := mgr.GenerateSpec()
		require.NoError(t, err)

		var spec map[string]any
		require.NoError(t, json.Unmarshal(specJSON, &spec))

		info, ok := spec["info"].(map[string]any)
		require.True(t, ok)

		contact, ok := info["contact"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "contact-123", contact["x-contact-id"])
	})

	t.Run("license extensions", func(t *testing.T) {
		t.Parallel()
		cfg := MustNew(
			WithTitle("Test API", "1.0.0"),
			WithLicense("MIT", "https://opensource.org/licenses/MIT"),
		)
		cfg.Info.License.Extensions = map[string]any{
			"x-license-id": "license-456",
		}

		mgr := NewManager(cfg)
		mgr.Register(http.MethodGet, "/test").Response(200, map[string]string{"id": "string"})
		specJSON, _, err := mgr.GenerateSpec()
		require.NoError(t, err)

		var spec map[string]any
		require.NoError(t, json.Unmarshal(specJSON, &spec))

		info, ok := spec["info"].(map[string]any)
		require.True(t, ok)

		license, ok := info["license"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "license-456", license["x-license-id"])
	})
}

func TestManager_Extensions_WithVersion31(t *testing.T) {
	t.Parallel()

	cfg := MustNew(
		WithTitle("Test API", "1.0.0"),
		WithVersion(Version31),
		WithExtension("x-custom-field", "value"),
	)

	mgr := NewManager(cfg)
	specJSON, _, err := mgr.GenerateSpec()
	require.NoError(t, err)

	var spec map[string]any
	require.NoError(t, json.Unmarshal(specJSON, &spec))

	// Verify OpenAPI version
	assert.Equal(t, "3.1.2", spec["openapi"])

	// Verify extension is present
	assert.Equal(t, "value", spec["x-custom-field"])
}

func TestManager_Extensions_ReservedPrefixes_Rejected(t *testing.T) {
	t.Parallel()

	// This should fail validation
	_, err := New(
		WithTitle("Test API", "1.0.0"),
		WithVersion(Version31),
		WithExtension("x-oai-custom", "value"),
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "reserved prefix")
}
