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

//go:build !integration

package config

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTestSourceWithError(t *testing.T) {
	t.Parallel()

	loadErr := errors.New("source load failed")
	src := TestSourceWithError(loadErr)
	require.NotNil(t, src)

	cfg, err := New(WithSource(src))
	require.NoError(t, err)

	err = cfg.Load(context.Background())
	require.Error(t, err)
	assert.ErrorContains(t, err, "source load failed")
}

func TestTestDumperWithError(t *testing.T) {
	t.Parallel()

	dumpErr := errors.New("dumper write failed")
	dumper := TestDumperWithError(dumpErr)
	require.NotNil(t, dumper)

	cfg, err := New(
		WithSource(TestSource(map[string]any{"foo": "bar"})),
		WithDumper(dumper),
	)
	require.NoError(t, err)
	require.NoError(t, cfg.Load(context.Background()))

	err = cfg.Dump(context.Background())
	require.Error(t, err)
	assert.ErrorContains(t, err, "dumper write failed")
}

func TestTestYAMLFile(t *testing.T) {
	t.Parallel()

	content := []byte("key: value\nnested:\n  num: 42")
	path := TestYAMLFile(t, content)
	require.NotEmpty(t, path)

	//nolint:gosec // G304: path is from TestYAMLFile (t.TempDir() + fixed name), not user input
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, content, data)
}

func TestTestJSONFile(t *testing.T) {
	t.Parallel()

	content := []byte(`{"key":"value","nested":{"num":42}}`)
	path := TestJSONFile(t, content)
	require.NotEmpty(t, path)

	//nolint:gosec // G304: path is from TestJSONFile (t.TempDir() + fixed name), not user input
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, content, data)
}

func TestTestTOMLFile(t *testing.T) {
	t.Parallel()

	content := []byte("key = \"value\"\n[nested]\nnum = 42")
	path := TestTOMLFile(t, content)
	require.NotEmpty(t, path)

	//nolint:gosec // G304: path is from TestTOMLFile (t.TempDir() + fixed name), not user input
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, content, data)
}

func TestTestConfigFromYAMLFile(t *testing.T) {
	t.Parallel()

	content := []byte("app: myapp\nport: 8080")
	cfg := TestConfigFromYAMLFile(t, content)
	require.NotNil(t, cfg)
	assert.Equal(t, "myapp", cfg.String("app"))
	assert.Equal(t, 8080, cfg.Int("port"))
}

func TestTestConfigFromJSONFile(t *testing.T) {
	t.Parallel()

	content := []byte(`{"app":"myapp","port":8080}`)
	cfg := TestConfigFromJSONFile(t, content)
	require.NotNil(t, cfg)
	assert.Equal(t, "myapp", cfg.String("app"))
	assert.Equal(t, 8080, cfg.Int("port"))
}

func TestTestConfigFromTOMLFile(t *testing.T) {
	t.Parallel()

	content := []byte("app = \"myapp\"\nport = 8080")
	cfg := TestConfigFromTOMLFile(t, content)
	require.NotNil(t, cfg)
	assert.Equal(t, "myapp", cfg.String("app"))
	assert.Equal(t, 8080, cfg.Int("port"))
}

func TestAssertConfigValue(t *testing.T) {
	t.Parallel()

	cfg := TestConfigLoaded(t, map[string]any{"foo": "bar", "num": 42})
	AssertConfigValue(t, cfg, "foo", "bar")
	AssertConfigValue(t, cfg, "num", 42)
}

func TestAssertConfigString(t *testing.T) {
	t.Parallel()

	cfg := TestConfigLoaded(t, map[string]any{"host": "localhost"})
	AssertConfigString(t, cfg, "host", "localhost")
}

func TestAssertConfigInt(t *testing.T) {
	t.Parallel()

	cfg := TestConfigLoaded(t, map[string]any{"port": 9090})
	AssertConfigInt(t, cfg, "port", 9090)
}

func TestAssertConfigBool(t *testing.T) {
	t.Parallel()

	cfg := TestConfigLoaded(t, map[string]any{"enabled": true})
	AssertConfigBool(t, cfg, "enabled", true)
}

func TestMockCodec(t *testing.T) {
	t.Parallel()

	t.Run("Decode and Encode succeed", func(t *testing.T) {
		t.Parallel()

		decodeCalled := false
		encodeCalled := false
		mock := NewMockCodec(
			func(data []byte, v any) error {
				decodeCalled = true
				return nil
			},
			func(v any) ([]byte, error) {
				encodeCalled = true
				return []byte("encoded"), nil
			},
		)

		var dst map[string]any
		err := mock.Decode([]byte("input"), &dst)
		require.NoError(t, err)
		assert.True(t, decodeCalled)

		out, err := mock.Encode(map[string]any{"x": 1})
		require.NoError(t, err)
		assert.True(t, encodeCalled)
		assert.Equal(t, []byte("encoded"), out)
	})

	t.Run("Decode returns error", func(t *testing.T) {
		t.Parallel()

		decodeErr := errors.New("decode failed")
		mock := NewMockCodec(
			func([]byte, any) error { return decodeErr },
			nil,
		)

		var dst map[string]any
		err := mock.Decode([]byte("x"), &dst)
		require.Error(t, err)
		assert.ErrorContains(t, err, "decode failed")
	})

	t.Run("Encode returns error", func(t *testing.T) {
		t.Parallel()

		encodeErr := errors.New("encode failed")
		mock := NewMockCodec(
			nil,
			func(any) ([]byte, error) { return nil, encodeErr },
		)

		_, err := mock.Encode(map[string]any{})
		require.Error(t, err)
		assert.ErrorContains(t, err, "encode failed")
	})
}
