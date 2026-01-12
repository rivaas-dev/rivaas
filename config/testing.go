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

package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// mockSource is a test implementation of the Source interface.
type mockSource struct {
	conf map[string]any
	err  error
}

// Load implements the Source interface for testing.
func (m *mockSource) Load(_ context.Context) (map[string]any, error) {
	return m.conf, m.err //nolint:nilnil // Test mock intentionally returns (nil, nil) for certain test cases
}

// MockDumper is a test implementation of the Dumper interface.
type MockDumper struct {
	called bool
	values *map[string]any
	err    error
}

// Dump implements the Dumper interface for testing.
func (m *MockDumper) Dump(_ context.Context, values *map[string]any) error {
	m.called = true
	m.values = values
	return m.err
}

// bindStruct is a test struct for binding tests.
type bindStruct struct {
	Foo string `config:"foo"`
	Bar int    `config:"bar"`
}

// TestSource creates a mock source for testing with the given configuration map.
func TestSource(conf map[string]any) Source {
	return &mockSource{conf: conf}
}

// TestSourceWithError creates a mock source that returns an error on Load.
func TestSourceWithError(err error) Source {
	return &mockSource{err: err}
}

// TestDumper creates a mock dumper for testing.
func TestDumper() *MockDumper {
	return &MockDumper{}
}

// TestDumperWithError creates a mock dumper that returns an error on Dump.
func TestDumperWithError(err error) *MockDumper {
	return &MockDumper{err: err}
}

// TestConfig creates a new Config instance with the given options for testing.
// It fails the test if creation fails.
func TestConfig(t *testing.T, opts ...Option) *Config {
	t.Helper()
	cfg, err := New(opts...)
	require.NoError(t, err, "failed to create test config")
	return cfg
}

// TestConfigWithSource creates a new Config instance with a mock source for testing.
func TestConfigWithSource(t *testing.T, conf map[string]any) *Config {
	t.Helper()
	return TestConfig(t, WithSource(TestSource(conf)))
}

// TestConfigLoaded creates and loads a Config instance with the given configuration.
// Note: Uses context.Background() for simplicity. For tests needing context control,
// create the config manually and call Load with t.Context().
func TestConfigLoaded(t *testing.T, conf map[string]any) *Config {
	t.Helper()
	cfg := TestConfigWithSource(t, conf)
	err := cfg.Load(context.Background())
	require.NoError(t, err, "failed to load test config")
	return cfg
}

// TestYAMLFile creates a temporary YAML file with the given content.
// The file is automatically cleaned up when the test completes.
func TestYAMLFile(t *testing.T, content []byte) string {
	t.Helper()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(filePath, content, 0o600)
	require.NoError(t, err, "failed to create test YAML file")
	return filePath
}

// TestJSONFile creates a temporary JSON file with the given content.
// The file is automatically cleaned up when the test completes.
func TestJSONFile(t *testing.T, content []byte) string {
	t.Helper()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "config.json")
	err := os.WriteFile(filePath, content, 0o600)
	require.NoError(t, err, "failed to create test JSON file")
	return filePath
}

// TestTOMLFile creates a temporary TOML file with the given content.
// The file is automatically cleaned up when the test completes.
func TestTOMLFile(t *testing.T, content []byte) string {
	t.Helper()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "config.toml")
	err := os.WriteFile(filePath, content, 0o600)
	require.NoError(t, err, "failed to create test TOML file")
	return filePath
}

// TestConfigFromYAMLFile creates a Config instance loaded from a temporary YAML file.
func TestConfigFromYAMLFile(t *testing.T, content []byte) *Config {
	t.Helper()
	filePath := TestYAMLFile(t, content)
	cfg, err := New(WithFile(filePath))
	require.NoError(t, err, "failed to create config from YAML file")
	err = cfg.Load(context.Background())
	require.NoError(t, err, "failed to load config from YAML file")
	return cfg
}

// TestConfigFromJSONFile creates a Config instance loaded from a temporary JSON file.
func TestConfigFromJSONFile(t *testing.T, content []byte) *Config {
	t.Helper()
	filePath := TestJSONFile(t, content)
	cfg, err := New(WithFile(filePath))
	require.NoError(t, err, "failed to create config from JSON file")
	err = cfg.Load(context.Background())
	require.NoError(t, err, "failed to load config from JSON file")
	return cfg
}

// TestConfigFromTOMLFile creates a Config instance loaded from a temporary TOML file.
func TestConfigFromTOMLFile(t *testing.T, content []byte) *Config {
	t.Helper()
	filePath := TestTOMLFile(t, content)
	cfg, err := New(WithFile(filePath))
	require.NoError(t, err, "failed to create config from TOML file")
	err = cfg.Load(context.Background())
	require.NoError(t, err, "failed to load config from TOML file")
	return cfg
}

// TestConfigWithBinding creates a Config instance with the given configuration and binding target.
func TestConfigWithBinding(t *testing.T, conf map[string]any, target any) *Config {
	t.Helper()
	cfg, err := New(WithSource(TestSource(conf)), WithBinding(target))
	require.NoError(t, err, "failed to create config with binding")
	err = cfg.Load(context.Background())
	require.NoError(t, err, "failed to load config with binding")
	return cfg
}

// TestConfigWithValidator creates a Config instance with the given configuration and validator.
func TestConfigWithValidator(t *testing.T, conf map[string]any, validator func(map[string]any) error) *Config {
	t.Helper()
	cfg, err := New(WithSource(TestSource(conf)), WithValidator(validator))
	require.NoError(t, err, "failed to create config with validator")
	return cfg
}

// AssertConfigValue asserts that a configuration value matches the expected value.
func AssertConfigValue(t *testing.T, cfg *Config, key string, expected any) {
	t.Helper()
	actual := cfg.Get(key)
	require.Equal(t, expected, actual, "config value mismatch for key %q", key)
}

// AssertConfigString asserts that a string configuration value matches the expected value.
func AssertConfigString(t *testing.T, cfg *Config, key, expected string) {
	t.Helper()
	actual := cfg.String(key)
	require.Equal(t, expected, actual, "config string mismatch for key %q", key)
}

// AssertConfigInt asserts that an integer configuration value matches the expected value.
func AssertConfigInt(t *testing.T, cfg *Config, key string, expected int) {
	t.Helper()
	actual := cfg.Int(key)
	require.Equal(t, expected, actual, "config int mismatch for key %q", key)
}

// AssertConfigBool asserts that a boolean configuration value matches the expected value.
func AssertConfigBool(t *testing.T, cfg *Config, key string, expected bool) {
	t.Helper()
	actual := cfg.Bool(key)
	require.Equal(t, expected, actual, "config bool mismatch for key %q", key)
}

// MockDecoder is a test decoder that can be configured to return specific values or errors.
type MockDecoder struct {
	DecodeFunc func(data []byte, v any) error
}

// Decode implements the codec.Decoder interface.
func (m *MockDecoder) Decode(data []byte, v any) error {
	if m.DecodeFunc != nil {
		return m.DecodeFunc(data, v)
	}
	return nil
}

// MockEncoder is a test encoder that can be configured to return specific values or errors.
type MockEncoder struct {
	EncodeFunc func(v any) ([]byte, error)
}

// Encode implements the codec.Encoder interface.
func (m *MockEncoder) Encode(v any) ([]byte, error) {
	if m.EncodeFunc != nil {
		return m.EncodeFunc(v)
	}
	return []byte{}, nil
}

// MockCodec is a test codec that implements both Encoder and Decoder.
type MockCodec struct {
	MockDecoder
	MockEncoder
}

// NewMockCodec creates a new mock codec for testing.
func NewMockCodec(decodeFunc func([]byte, any) error, encodeFunc func(any) ([]byte, error)) *MockCodec {
	return &MockCodec{
		MockDecoder: MockDecoder{DecodeFunc: decodeFunc},
		MockEncoder: MockEncoder{EncodeFunc: encodeFunc},
	}
}
