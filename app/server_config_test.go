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
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestServerConfig_Validate tests the Validate() method on serverConfig.
func TestServerConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  *serverConfig
		wantErr bool
		check   func(t *testing.T, err error)
	}{
		{
			name: "valid configuration",
			config: &serverConfig{
				readTimeout:       10 * time.Second,
				writeTimeout:      10 * time.Second,
				idleTimeout:       60 * time.Second,
				readHeaderTimeout: 2 * time.Second,
				maxHeaderBytes:    1 << 20, // 1MB
				shutdownTimeout:   30 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "read timeout exceeds write timeout",
			config: &serverConfig{
				readTimeout:       15 * time.Second,
				writeTimeout:      10 * time.Second, // Read > Write (invalid)
				idleTimeout:       60 * time.Second,
				readHeaderTimeout: 2 * time.Second,
				maxHeaderBytes:    1 << 20,
				shutdownTimeout:   30 * time.Second,
			},
			wantErr: true,
			check: func(t *testing.T, err error) {
				t.Helper()
				var ve *ValidationError
				require.ErrorAs(t, err, &ve, "should return ValidationErrors")
				assert.Contains(t, err.Error(), "read timeout should not exceed write timeout")
			},
		},
		{
			name: "shutdown timeout too short",
			config: &serverConfig{
				readTimeout:       10 * time.Second,
				writeTimeout:      10 * time.Second,
				idleTimeout:       60 * time.Second,
				readHeaderTimeout: 2 * time.Second,
				maxHeaderBytes:    1 << 20,
				shutdownTimeout:   100 * time.Millisecond, // Too short (invalid)
			},
			wantErr: true,
			check: func(t *testing.T, err error) {
				t.Helper()
				var ve *ValidationError
				require.ErrorAs(t, err, &ve, "should return ValidationErrors")
				assert.Contains(t, err.Error(), "must be at least 1 second")
			},
		},
		{
			name: "max header bytes too small",
			config: &serverConfig{
				readTimeout:       10 * time.Second,
				writeTimeout:      10 * time.Second,
				idleTimeout:       60 * time.Second,
				readHeaderTimeout: 2 * time.Second,
				maxHeaderBytes:    512, // Too small (invalid)
				shutdownTimeout:   30 * time.Second,
			},
			wantErr: true,
			check: func(t *testing.T, err error) {
				t.Helper()
				var ve *ValidationError
				require.ErrorAs(t, err, &ve, "should return ValidationErrors")
				assert.Contains(t, err.Error(), "must be at least 1KB")
			},
		},
		{
			name: "negative read timeout",
			config: &serverConfig{
				readTimeout:       -1 * time.Second,
				writeTimeout:      10 * time.Second,
				idleTimeout:       60 * time.Second,
				readHeaderTimeout: 2 * time.Second,
				maxHeaderBytes:    1 << 20,
				shutdownTimeout:   30 * time.Second,
			},
			wantErr: true,
			check: func(t *testing.T, err error) {
				t.Helper()
				var ve *ValidationError
				require.ErrorAs(t, err, &ve, "should return ValidationErrors")
				assert.Contains(t, err.Error(), "server.readTimeout")
			},
		},
		{
			name: "zero write timeout",
			config: &serverConfig{
				readTimeout:       10 * time.Second,
				writeTimeout:      0, // Invalid
				idleTimeout:       60 * time.Second,
				readHeaderTimeout: 2 * time.Second,
				maxHeaderBytes:    1 << 20,
				shutdownTimeout:   30 * time.Second,
			},
			wantErr: true,
			check: func(t *testing.T, err error) {
				t.Helper()
				var ve *ValidationError
				require.ErrorAs(t, err, &ve, "should return ValidationErrors")
				assert.Contains(t, err.Error(), "server.writeTimeout")
			},
		},
		{
			name: "multiple validation errors",
			config: &serverConfig{
				readTimeout:       15 * time.Second,
				writeTimeout:      10 * time.Second,       // Read > Write
				idleTimeout:       -5 * time.Second,       // Negative
				readHeaderTimeout: 0,                      // Zero
				maxHeaderBytes:    512,                    // Too small
				shutdownTimeout:   100 * time.Millisecond, // Too short
			},
			wantErr: true,
			check: func(t *testing.T, err error) {
				t.Helper()
				var ve *ValidationError
				require.ErrorAs(t, err, &ve, "should return ValidationErrors")
				// Should have multiple errors
				assert.Greater(t, len(ve.Errors), 1, "should have multiple validation errors")
			},
		},
		{
			name: "read timeout equals write timeout (valid)",
			config: &serverConfig{
				readTimeout:       10 * time.Second,
				writeTimeout:      10 * time.Second, // Equal is valid
				idleTimeout:       60 * time.Second,
				readHeaderTimeout: 2 * time.Second,
				maxHeaderBytes:    1 << 20,
				shutdownTimeout:   30 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "write timeout greater than read timeout (valid)",
			config: &serverConfig{
				readTimeout:       10 * time.Second,
				writeTimeout:      15 * time.Second, // Write > Read (valid)
				idleTimeout:       60 * time.Second,
				readHeaderTimeout: 2 * time.Second,
				maxHeaderBytes:    1 << 20,
				shutdownTimeout:   30 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "exactly 1KB max header bytes (valid)",
			config: &serverConfig{
				readTimeout:       10 * time.Second,
				writeTimeout:      10 * time.Second,
				idleTimeout:       60 * time.Second,
				readHeaderTimeout: 2 * time.Second,
				maxHeaderBytes:    1024, // Exactly 1KB (valid)
				shutdownTimeout:   30 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "exactly 1 second shutdown timeout (valid)",
			config: &serverConfig{
				readTimeout:       10 * time.Second,
				writeTimeout:      10 * time.Second,
				idleTimeout:       60 * time.Second,
				readHeaderTimeout: 2 * time.Second,
				maxHeaderBytes:    1 << 20,
				shutdownTimeout:   1 * time.Second, // Exactly 1s (valid)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			errs := tt.config.Validate()
			if tt.wantErr {
				assert.NotNil(t, errs, "should return validation errors")
				assert.True(t, errs.HasErrors(), "should have errors")
				if tt.check != nil {
					tt.check(t, errs.ToError())
				}
			} else {
				assert.NoError(t, errs.ToError(), "should not have errors")
			}
		})
	}
}

// TestServerConfig_Validate_Integration tests that serverConfig.Validate()
// is properly called from config.validate().
func TestServerConfig_Validate_Integration(t *testing.T) {
	t.Parallel()

	t.Run("server config validation errors are included in app validation", func(t *testing.T) {
		t.Parallel()
		app, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithServer(
				WithReadTimeout(15*time.Second),
				WithWriteTimeout(10*time.Second), // Read > Write (invalid)
			),
		)

		require.Error(t, err, "should return validation error")
		assert.Nil(t, app, "app should be nil on validation error")
		assert.Contains(t, err.Error(), "read timeout should not exceed write timeout")
	})

	t.Run("multiple server config errors are collected", func(t *testing.T) {
		t.Parallel()
		app, err := New(
			WithServiceName("test"),
			WithServiceVersion("1.0.0"),
			WithServer(
				WithReadTimeout(15*time.Second),
				WithWriteTimeout(10*time.Second),          // Read > Write
				WithMaxHeaderBytes(512),                   // Too small
				WithShutdownTimeout(100*time.Millisecond), // Too short
			),
		)

		require.Error(t, err, "should return validation error")
		assert.Nil(t, app, "app should be nil on validation error")

		var ve *ValidationError
		if errors.As(err, &ve) {
			// Should have multiple server config errors
			serverErrorCount := 0
			for _, e := range ve.Errors {
				if e.Field == "server.readTimeout" || e.Field == "server.maxHeaderBytes" ||
					e.Field == "server.shutdownTimeout" {

					serverErrorCount++
				}
			}
			assert.GreaterOrEqual(t, serverErrorCount, 2, "should have multiple server config errors")
		}
	})
}
