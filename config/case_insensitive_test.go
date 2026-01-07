// Copyright 2025 The Rivaas Authors
// Copyright 2025 Company.info B.V.
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

package config

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/config/codec"
)

func TestCaseInsensitiveMerging(t *testing.T) {
	t.Parallel()

	// Test data with mixed case keys
	config1 := []byte(`{
		"Server": {
			"Host": "localhost",
			"Port": 8080
		},
		"Database": {
			"Name": "testdb"
		}
	}`)

	config2 := []byte(`{
		"server": {
			"host": "example.com",
			"port": 9090
		},
		"database": {
			"name": "prod"
		}
	}`)

	// Create configuration with both sources
	cfg, err := New(
		WithContentSource(config1, codec.TypeJSON),
		WithContentSource(config2, codec.TypeJSON),
	)
	require.NoError(t, err)

	// Load configuration
	err = cfg.Load(context.Background())
	require.NoError(t, err)

	tests := []struct {
		name    string
		key     string
		wantStr string
		wantInt int
		getType string // "string" or "int"
	}{
		{
			name:    "server.host lowercase",
			key:     "server.host",
			wantStr: "example.com",
			getType: "string",
		},
		{
			name:    "Server.Host mixed case",
			key:     "Server.Host",
			wantStr: "example.com",
			getType: "string",
		},
		{
			name:    "SERVER.HOST uppercase",
			key:     "SERVER.HOST",
			wantStr: "example.com",
			getType: "string",
		},
		{
			name:    "server.port lowercase",
			key:     "server.port",
			wantInt: 9090,
			getType: "int",
		},
		{
			name:    "Server.Port mixed case",
			key:     "Server.Port",
			wantInt: 9090,
			getType: "int",
		},
		{
			name:    "SERVER.PORT uppercase",
			key:     "SERVER.PORT",
			wantInt: 9090,
			getType: "int",
		},
		{
			name:    "database.name lowercase",
			key:     "database.name",
			wantStr: "prod",
			getType: "string",
		},
		{
			name:    "Database.Name mixed case",
			key:     "Database.Name",
			wantStr: "prod",
			getType: "string",
		},
		{
			name:    "DATABASE.NAME uppercase",
			key:     "DATABASE.NAME",
			wantStr: "prod",
			getType: "string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			switch tt.getType {
			case "string":
				assert.Equal(t, tt.wantStr, cfg.String(tt.key))
			case "int":
				assert.Equal(t, tt.wantInt, cfg.Int(tt.key))
			}
		})
	}
}

func TestNormalizeMapKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    map[string]any
		expected map[string]any
	}{
		{
			name: "normalizes all keys to lowercase",
			input: map[string]any{
				"Server": map[string]any{
					"Host": "localhost",
					"Port": 8080,
				},
				"Database": map[string]any{
					"Name": "testdb",
					"Settings": map[string]any{
						"MaxConnections": 100,
					},
				},
			},
			expected: map[string]any{
				"server": map[string]any{
					"host": "localhost",
					"port": 8080,
				},
				"database": map[string]any{
					"name": "testdb",
					"settings": map[string]any{
						"maxconnections": 100,
					},
				},
			},
		},
		{
			name: "handles already lowercase keys",
			input: map[string]any{
				"server": "localhost",
				"port":   8080,
			},
			expected: map[string]any{
				"server": "localhost",
				"port":   8080,
			},
		},
		{
			name: "handles uppercase keys",
			input: map[string]any{
				"SERVER": "localhost",
				"PORT":   8080,
			},
			expected: map[string]any{
				"server": "localhost",
				"port":   8080,
			},
		},
		{
			name: "handles mixed case keys",
			input: map[string]any{
				"MyServer": "localhost",
				"MyPort":   8080,
			},
			expected: map[string]any{
				"myserver": "localhost",
				"myport":   8080,
			},
		},
		{
			name:     "handles empty map",
			input:    map[string]any{},
			expected: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			normalized := normalizeMapKeys(tt.input)
			assert.Equal(t, tt.expected, normalized)
		})
	}
}
