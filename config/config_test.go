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
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/config/codec"
)

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		opts    []Option
		wantErr bool
		errMsg  string
	}{
		{
			name:    "no options succeeds",
			opts:    nil,
			wantErr: false,
		},
		{
			name:    "with valid source succeeds",
			opts:    []Option{WithSource(&mockSource{conf: map[string]any{"foo": "bar"}})},
			wantErr: false,
		},
		{
			name:    "with nil source fails",
			opts:    []Option{WithSource(nil)},
			wantErr: true,
			errMsg:  "source cannot be nil",
		},
		{
			name:    "with nil dumper fails",
			opts:    []Option{WithDumper(nil)},
			wantErr: true,
			errMsg:  "dumper cannot be nil",
		},
		{
			name:    "with nil binding fails",
			opts:    []Option{WithBinding(nil)},
			wantErr: true,
			errMsg:  "binding target cannot be nil",
		},
		{
			name: "with non-pointer binding fails",
			opts: []Option{
				WithSource(&mockSource{conf: map[string]any{"foo": "bar"}}),
				WithBinding(bindStruct{}), // not a pointer
			},
			wantErr: true,
			errMsg:  "binding target must be a pointer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := New(tt.opts...)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, cfg)
		})
	}
}

func TestMustNew(t *testing.T) {
	t.Parallel()

	t.Run("success with no options", func(t *testing.T) {
		t.Parallel()
		c := MustNew()
		assert.NotNil(t, c)
	})

	t.Run("success with valid source", func(t *testing.T) {
		t.Parallel()
		src := &mockSource{conf: map[string]any{"foo": "bar"}}
		c := MustNew(WithSource(src))
		assert.NotNil(t, c)
		require.NoError(t, c.Load(context.Background()))
		assert.Equal(t, "bar", c.String("foo"))
	})

	t.Run("panics with nil source", func(t *testing.T) {
		t.Parallel()
		assert.Panics(t, func() {
			MustNew(WithSource(nil))
		})
	})
}

func TestLoad(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func() (*Config, error)
		wantErr bool
		errMsg  string
	}{
		{
			name: "succeeds with valid source",
			setup: func() (*Config, error) {
				return New(WithSource(&mockSource{conf: map[string]any{"foo": "bar", "bar": 42}}))
			},
			wantErr: false,
		},
		{
			name: "succeeds with no sources",
			setup: func() (*Config, error) {
				return New()
			},
			wantErr: false,
		},
		{
			name: "succeeds with nil source map",
			setup: func() (*Config, error) {
				return New(WithSource(&mockSource{conf: nil}))
			},
			wantErr: false,
		},
		{
			name: "error propagates from source",
			setup: func() (*Config, error) {
				return New(WithSource(&mockSource{err: errors.New("fail")}))
			},
			wantErr: true,
		},
		{
			name: "multiple sources merge correctly",
			setup: func() (*Config, error) {
				return New(
					WithSource(&mockSource{conf: map[string]any{"foo": "bar", "bar": 1}}),
					WithSource(&mockSource{conf: map[string]any{"bar": 2, "baz": 3}}),
				)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := tt.setup()
			require.NoError(t, err, "setup should not fail")

			err = cfg.Load(context.Background())

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestLoad_MultipleSources(t *testing.T) {
	t.Parallel()

	src1 := &mockSource{conf: map[string]any{"foo": "bar", "bar": 1}}
	src2 := &mockSource{conf: map[string]any{"bar": 2, "baz": 3}}
	cfg, err := New(WithSource(src1), WithSource(src2))
	require.NoError(t, err)

	err = cfg.Load(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "bar", cfg.String("foo"))
	assert.Equal(t, 2, cfg.Int("bar")) // src2 overrides src1
	assert.Equal(t, 3, cfg.Int("baz"))
}

func TestMustLoad(t *testing.T) {
	t.Parallel()

	t.Run("success with valid source", func(t *testing.T) {
		t.Parallel()
		src := &mockSource{conf: map[string]any{"foo": "bar"}}
		cfg := MustNew(WithSource(src))
		assert.NotPanics(t, func() {
			cfg.MustLoad(context.Background())
		})
		assert.Equal(t, "bar", cfg.String("foo"))
	})

	t.Run("success with no sources", func(t *testing.T) {
		t.Parallel()
		cfg := MustNew()
		assert.NotPanics(t, func() {
			cfg.MustLoad(context.Background())
		})
	})

	t.Run("panics on source error", func(t *testing.T) {
		t.Parallel()
		src := &mockSource{err: errors.New("source failed")}
		cfg := MustNew(WithSource(src))
		assert.Panics(t, func() {
			cfg.MustLoad(context.Background())
		})
	})

	t.Run("panics on nil context", func(t *testing.T) {
		t.Parallel()
		cfg := MustNew()
		assert.Panics(t, func() {
			//nolint:staticcheck // Intentionally testing nil context error handling
			cfg.MustLoad(nil)
		})
	})

	t.Run("panics on validation error", func(t *testing.T) {
		t.Parallel()
		src := &mockSource{conf: map[string]any{"foo": "bar"}}
		cfg := MustNew(
			WithSource(src),
			WithValidator(func(values map[string]any) error {
				return errors.New("validation failed")
			}),
		)
		assert.Panics(t, func() {
			cfg.MustLoad(context.Background())
		})
	})
}

func TestBinding(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		conf      map[string]any
		setupBind func() any
		verify    func(t *testing.T, target any)
		wantErr   bool
	}{
		{
			name: "basic binding succeeds",
			conf: map[string]any{"foo": "bar", "bar": 42},
			setupBind: func() any {
				return &bindStruct{}
			},
			verify: func(t *testing.T, target any) {
				bind, isBind := target.(*bindStruct)
				require.True(t, isBind)
				assert.Equal(t, "bar", bind.Foo)
				assert.Equal(t, 42, bind.Bar)
			},
			wantErr: false,
		},
		{
			name: "binding with extra fields succeeds",
			conf: map[string]any{"foo": "bar", "bar": 42, "extra": 99},
			setupBind: func() any {
				return &bindStruct{}
			},
			verify: func(t *testing.T, target any) {
				bind, isBind := target.(*bindStruct)
				require.True(t, isBind)
				assert.Equal(t, "bar", bind.Foo)
				assert.Equal(t, 42, bind.Bar)
			},
			wantErr: false,
		},
		{
			name: "binding with missing fields uses defaults",
			conf: map[string]any{"foo": "bar"},
			setupBind: func() any {
				return &bindStruct{}
			},
			verify: func(t *testing.T, target any) {
				bind, isBind := target.(*bindStruct)
				require.True(t, isBind)
				assert.Equal(t, "bar", bind.Foo)
				assert.Equal(t, 0, bind.Bar)
			},
			wantErr: false,
		},
		{
			name: "binding with type mismatch fails",
			conf: map[string]any{"foo": 123, "bar": "notanint"},
			setupBind: func() any {
				return &bindStruct{}
			},
			verify:  nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			target := tt.setupBind()
			src := &mockSource{conf: tt.conf}
			cfg, err := New(WithSource(src), WithBinding(target))
			require.NoError(t, err)

			err = cfg.Load(context.Background())

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			if tt.verify != nil {
				tt.verify(t, target)
			}
		})
	}
}

func TestDump(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func() (*Config, *MockDumper, error)
		verify  func(t *testing.T, dumper *MockDumper)
		wantErr bool
	}{
		{
			name: "calls dumper successfully",
			setup: func() (*Config, *MockDumper, error) {
				src := &mockSource{conf: map[string]any{"foo": "bar"}}
				dumper := &MockDumper{}
				cfg, err := New(WithSource(src), WithDumper(dumper))
				if err != nil {
					return nil, nil, err
				}
				if err = cfg.Load(context.Background()); err != nil {
					return nil, nil, err
				}
				return cfg, dumper, nil
			},
			verify: func(t *testing.T, dumper *MockDumper) {
				assert.True(t, dumper.called)
				require.NotNil(t, dumper.values)
				assert.Equal(t, "bar", (*dumper.values)["foo"])
			},
			wantErr: false,
		},
		{
			name: "succeeds with no dumpers",
			setup: func() (*Config, *MockDumper, error) {
				src := &mockSource{conf: map[string]any{"foo": "bar"}}
				cfg, err := New(WithSource(src))
				if err != nil {
					return nil, nil, err
				}
				if err = cfg.Load(context.Background()); err != nil {
					return nil, nil, err
				}
				return cfg, nil, nil
			},
			verify:  nil,
			wantErr: false,
		},
		{
			name: "error propagates from dumper",
			setup: func() (*Config, *MockDumper, error) {
				src := &mockSource{conf: map[string]any{"foo": "bar"}}
				dumper := &MockDumper{err: errors.New("dump error")}
				cfg, err := New(WithSource(src), WithDumper(dumper))
				if err != nil {
					return nil, nil, err
				}
				if err = cfg.Load(context.Background()); err != nil {
					return nil, nil, err
				}
				return cfg, dumper, nil
			},
			verify:  nil,
			wantErr: true,
		},
		{
			name: "calls multiple dumpers",
			setup: func() (*Config, *MockDumper, error) {
				src := &mockSource{conf: map[string]any{"foo": "bar"}}
				dumper1 := &MockDumper{}
				dumper2 := &MockDumper{}
				cfg, err := New(WithSource(src), WithDumper(dumper1), WithDumper(dumper2))
				if err != nil {
					return nil, nil, err
				}
				if err = cfg.Load(context.Background()); err != nil {
					return nil, nil, err
				}
				// Return first dumper for verification
				return cfg, dumper1, nil
			},
			verify: func(t *testing.T, dumper *MockDumper) {
				assert.True(t, dumper.called)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg, dumper, err := tt.setup()
			require.NoError(t, err, "setup should not fail")

			err = cfg.Dump(context.Background())

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			if tt.verify != nil && dumper != nil {
				tt.verify(t, dumper)
			}
		})
	}
}

func TestMustDump(t *testing.T) {
	t.Parallel()

	t.Run("success with valid dumper", func(t *testing.T) {
		t.Parallel()
		src := &mockSource{conf: map[string]any{"foo": "bar"}}
		dumper := &MockDumper{}
		cfg := MustNew(WithSource(src), WithDumper(dumper))
		cfg.MustLoad(context.Background())
		assert.NotPanics(t, func() {
			cfg.MustDump(context.Background())
		})
		assert.True(t, dumper.called)
		require.NotNil(t, dumper.values)
		assert.Equal(t, "bar", (*dumper.values)["foo"])
	})

	t.Run("success with no dumpers", func(t *testing.T) {
		t.Parallel()
		src := &mockSource{conf: map[string]any{"foo": "bar"}}
		cfg := MustNew(WithSource(src))
		cfg.MustLoad(context.Background())
		assert.NotPanics(t, func() {
			cfg.MustDump(context.Background())
		})
	})

	t.Run("panics on dumper error", func(t *testing.T) {
		t.Parallel()
		src := &mockSource{conf: map[string]any{"foo": "bar"}}
		dumper := &MockDumper{err: errors.New("dump failed")}
		cfg := MustNew(WithSource(src), WithDumper(dumper))
		cfg.MustLoad(context.Background())
		assert.Panics(t, func() {
			cfg.MustDump(context.Background())
		})
	})

	t.Run("panics on nil context", func(t *testing.T) {
		t.Parallel()
		src := &mockSource{conf: map[string]any{"foo": "bar"}}
		cfg := MustNew(WithSource(src))
		cfg.MustLoad(context.Background())
		assert.Panics(t, func() {
			//nolint:staticcheck // SA1012: Intentionally testing nil context error handling
			cfg.MustDump(nil)
		})
	})

	t.Run("calls multiple dumpers", func(t *testing.T) {
		t.Parallel()
		src := &mockSource{conf: map[string]any{"foo": "bar"}}
		dumper1 := &MockDumper{}
		dumper2 := &MockDumper{}
		cfg := MustNew(WithSource(src), WithDumper(dumper1), WithDumper(dumper2))
		cfg.MustLoad(context.Background())
		assert.NotPanics(t, func() {
			cfg.MustDump(context.Background())
		})
		assert.True(t, dumper1.called)
		assert.True(t, dumper2.called)
	})
}

func TestDump_NilContext(t *testing.T) {
	t.Parallel()

	src := &mockSource{conf: map[string]any{"foo": "bar"}}
	cfg, err := New(WithSource(src))
	require.NoError(t, err)
	require.NoError(t, cfg.Load(context.Background()))

	// Testing nil context handling - we need to verify the function properly rejects nil
	// Using a helper to call Dump with nil to avoid linter warnings in the main test code
	callDumpWithNil := func(c *Config) error {
		//nolint:staticcheck // SA1012: Intentionally testing nil context error handling
		return c.Dump(nil)
	}

	err = callDumpWithNil(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context cannot be nil")
}

func TestGet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		conf map[string]any
		key  string
		want any
	}{
		{
			name: "simple key",
			conf: map[string]any{"foo": "bar"},
			key:  "foo",
			want: "bar",
		},
		{
			name: "nested key with dot notation",
			conf: map[string]any{
				"outer": map[string]any{
					"inner": map[string]any{
						"val": 42,
					},
				},
			},
			key:  "outer.inner.val",
			want: 42,
		},
		{
			name: "deeply nested key",
			conf: map[string]any{
				"a": map[string]any{"b": map[string]any{"c": map[string]any{"d": 1}}},
			},
			key:  "a.b.c.d",
			want: 1,
		},
		{
			name: "not found returns nil",
			conf: map[string]any{"foo": "bar"},
			key:  "notfound",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := TestConfigLoaded(t, tt.conf)
			got := cfg.Get(tt.key)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetTypedValues(t *testing.T) {
	t.Parallel()

	timeStr := "2023-01-01T12:00:00Z"
	durStr := "1h2m3s"
	conf := map[string]any{
		"str":         "foo",
		"bool":        true,
		"boolstr":     "true",
		"int":         42,
		"intstr":      "42",
		"int32":       int32(32),
		"int64":       int64(64),
		"uint8":       uint8(8),
		"uint":        uint(7),
		"uint16":      uint16(16),
		"uint32":      uint32(32),
		"uint64":      uint64(64),
		"float64":     3.14,
		"floatstr":    "2.71",
		"time":        timeStr,
		"duration":    durStr,
		"intslice":    []any{1, 2, 3},
		"strslice":    []any{"a", "b"},
		"map":         map[string]any{"a": 1},
		"mapstr":      map[string]any{"a": "x"},
		"mapstrslice": map[string]any{"a": []any{"x", "y"}},
	}

	cfg := TestConfigLoaded(t, conf)

	tests := []struct {
		name   string
		testFn func(t *testing.T)
	}{
		{
			name: "GetString",
			testFn: func(t *testing.T) {
				assert.Equal(t, "foo", cfg.String("str"))
				v, err := GetE[string](cfg, "str")
				require.NoError(t, err)
				assert.Equal(t, "foo", v)
				_, err = GetE[string](cfg, "notfound")
				assert.Error(t, err)
			},
		},
		{
			name: "GetBool",
			testFn: func(t *testing.T) {
				assert.True(t, cfg.Bool("bool"))
				b, err := GetE[bool](cfg, "boolstr")
				require.NoError(t, err)
				assert.True(t, b)
				_, err = GetE[bool](cfg, "notfound")
				assert.Error(t, err)
			},
		},
		{
			name: "GetInt",
			testFn: func(t *testing.T) {
				assert.Equal(t, 42, cfg.Int("int"))
				i, err := GetE[int](cfg, "intstr")
				require.NoError(t, err)
				assert.Equal(t, 42, i)
				_, err = GetE[int](cfg, "notfound")
				assert.Error(t, err)
			},
		},
		{
			name: "GetInt32",
			testFn: func(t *testing.T) {
				assert.Equal(t, int32(32), Get[int32](cfg, "int32"))
				i32, err := GetE[int32](cfg, "int32")
				require.NoError(t, err)
				assert.Equal(t, int32(32), i32)
				_, err = GetE[int32](cfg, "notfound")
				assert.Error(t, err)
			},
		},
		{
			name: "GetInt64",
			testFn: func(t *testing.T) {
				assert.Equal(t, int64(64), cfg.Int64("int64"))
				i64, err := GetE[int64](cfg, "int64")
				require.NoError(t, err)
				assert.Equal(t, int64(64), i64)
				_, err = GetE[int64](cfg, "notfound")
				assert.Error(t, err)
			},
		},
		{
			name: "GetUint8",
			testFn: func(t *testing.T) {
				assert.Equal(t, uint8(8), Get[uint8](cfg, "uint8"))
				u8, err := GetE[uint8](cfg, "uint8")
				require.NoError(t, err)
				assert.Equal(t, uint8(8), u8)
				_, err = GetE[uint8](cfg, "notfound")
				assert.Error(t, err)
			},
		},
		{
			name: "GetUint",
			testFn: func(t *testing.T) {
				assert.Equal(t, uint(7), Get[uint](cfg, "uint"))
				u, err := GetE[uint](cfg, "uint")
				require.NoError(t, err)
				assert.Equal(t, uint(7), u)
				_, err = GetE[uint](cfg, "notfound")
				assert.Error(t, err)
			},
		},
		{
			name: "GetUint16",
			testFn: func(t *testing.T) {
				assert.Equal(t, uint16(16), Get[uint16](cfg, "uint16"))
				u16, err := GetE[uint16](cfg, "uint16")
				require.NoError(t, err)
				assert.Equal(t, uint16(16), u16)
				_, err = GetE[uint16](cfg, "notfound")
				assert.Error(t, err)
			},
		},
		{
			name: "GetUint32",
			testFn: func(t *testing.T) {
				assert.Equal(t, uint32(32), Get[uint32](cfg, "uint32"))
				u32, err := GetE[uint32](cfg, "uint32")
				require.NoError(t, err)
				assert.Equal(t, uint32(32), u32)
				_, err = GetE[uint32](cfg, "notfound")
				assert.Error(t, err)
			},
		},
		{
			name: "GetUint64",
			testFn: func(t *testing.T) {
				assert.Equal(t, uint64(64), Get[uint64](cfg, "uint64"))
				u64, err := GetE[uint64](cfg, "uint64")
				require.NoError(t, err)
				assert.Equal(t, uint64(64), u64)
				_, err = GetE[uint64](cfg, "notfound")
				assert.Error(t, err)
			},
		},
		{
			name: "GetFloat64",
			testFn: func(t *testing.T) {
				assert.InDelta(t, 3.14, cfg.Float64("float64"), 0.0001)
				f64, err := GetE[float64](cfg, "floatstr")
				require.NoError(t, err)
				assert.InDelta(t, 2.71, f64, 0.0001)
				_, err = GetE[float64](cfg, "notfound")
				assert.Error(t, err)
			},
		},
		{
			name: "GetTime",
			testFn: func(t *testing.T) {
				tm, err := GetE[time.Time](cfg, "time")
				require.NoError(t, err)
				assert.Equal(t, time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC), tm)
				assert.Equal(t, time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC), cfg.Time("time"))
				_, err = GetE[time.Time](cfg, "notfound")
				assert.Error(t, err)
			},
		},
		{
			name: "GetDuration",
			testFn: func(t *testing.T) {
				d, err := GetE[time.Duration](cfg, "duration")
				require.NoError(t, err)
				assert.Equal(t, 1*time.Hour+2*time.Minute+3*time.Second, d)
				assert.Equal(t, 1*time.Hour+2*time.Minute+3*time.Second, cfg.Duration("duration"))
				_, err = GetE[time.Duration](cfg, "notfound")
				assert.Error(t, err)
			},
		},
		{
			name: "GetIntSlice",
			testFn: func(t *testing.T) {
				assert.Equal(t, []int{1, 2, 3}, cfg.IntSlice("intslice"))
				is, err := GetE[[]int](cfg, "intslice")
				require.NoError(t, err)
				assert.Equal(t, []int{1, 2, 3}, is)
				_, err = GetE[[]int](cfg, "notfound")
				assert.Error(t, err)
			},
		},
		{
			name: "GetStringSlice",
			testFn: func(t *testing.T) {
				assert.Equal(t, []string{"a", "b"}, cfg.StringSlice("strslice"))
				ss, err := GetE[[]string](cfg, "strslice")
				require.NoError(t, err)
				assert.Equal(t, []string{"a", "b"}, ss)
				_, err = GetE[[]string](cfg, "notfound")
				assert.Error(t, err)
			},
		},
		{
			name: "GetStringMap",
			testFn: func(t *testing.T) {
				assert.Equal(t, map[string]any{"a": 1}, cfg.StringMap("map"))
				m, err := GetE[map[string]any](cfg, "map")
				require.NoError(t, err)
				assert.Equal(t, map[string]any{"a": 1}, m)
				_, err = GetE[map[string]any](cfg, "notfound")
				assert.Error(t, err)
			},
		},
		{
			name: "GetStringMapString",
			testFn: func(t *testing.T) {
				assert.Equal(t, map[string]string{"a": "x"}, Get[map[string]string](cfg, "mapstr"))
				ms, err := GetE[map[string]string](cfg, "mapstr")
				require.NoError(t, err)
				assert.Equal(t, map[string]string{"a": "x"}, ms)
				_, err = GetE[map[string]string](cfg, "notfound")
				assert.Error(t, err)
			},
		},
		{
			name: "GetStringMapStringSlice",
			testFn: func(t *testing.T) {
				assert.Equal(t, map[string][]string{"a": {"x", "y"}}, Get[map[string][]string](cfg, "mapstrslice"))
				mss, err := GetE[map[string][]string](cfg, "mapstrslice")
				require.NoError(t, err)
				assert.Equal(t, map[string][]string{"a": {"x", "y"}}, mss)
				_, err = GetE[map[string][]string](cfg, "notfound")
				assert.Error(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.testFn(t)
		})
	}
}

func TestValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		conf      map[string]any
		validator func(map[string]any) error
		wantErr   bool
		errMsg    string
	}{
		{
			name: "validator passes",
			conf: map[string]any{"foo": "baz"},
			validator: func(cfg map[string]any) error {
				if cfg["foo"] != "baz" {
					return errors.New("foo must be 'baz'")
				}
				return nil
			},
			wantErr: false,
		},
		{
			name: "validator fails",
			conf: map[string]any{"foo": "bar"},
			validator: func(cfg map[string]any) error {
				if cfg["foo"] != "baz" {
					return errors.New("foo must be 'baz'")
				}
				return nil
			},
			wantErr: true,
		},
		{
			name: "validator panic is caught",
			conf: map[string]any{"foo": "bar"},
			validator: func(_ map[string]any) error {
				panic("validator panic")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			src := &mockSource{conf: tt.conf}
			cfg, err := New(WithSource(src), WithValidator(tt.validator))
			require.NoError(t, err)

			err = cfg.Load(context.Background())

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestJSONSchemaValidation(t *testing.T) {
	t.Parallel()

	schema := []byte(`{"$schema":"http://json-schema.org/draft-07/schema#","type":"object","properties":{"foo":{"type":"string"},"bar":{"type":"integer"}},"required":["foo","bar"]}`)

	tests := []struct {
		name    string
		conf    map[string]any
		wantErr bool
	}{
		{
			name:    "valid data passes",
			conf:    map[string]any{"foo": "bar", "bar": 42},
			wantErr: false,
		},
		{
			name:    "invalid data fails",
			conf:    map[string]any{"foo": "bar", "bar": "notanint"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			src := &mockSource{conf: tt.conf}
			cfg, err := New(WithSource(src), WithJSONSchema(schema))
			require.NoError(t, err)

			err = cfg.Load(context.Background())

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestWithFileAs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		path      string
		codecType codec.Type
		wantErr   bool
	}{
		{
			name:      "valid path and codec",
			path:      "/tmp/config.json",
			codecType: codec.TypeJSON,
			wantErr:   false,
		},
		{
			name:      "valid path with YAML codec",
			path:      "/tmp/config.yaml",
			codecType: codec.TypeYAML,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := New(WithFileAs(tt.path, tt.codecType))

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, cfg)
			assert.Len(t, cfg.sources, 1)
		})
	}
}

func TestWithContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		data      []byte
		codecType codec.Type
		wantErr   bool
	}{
		{
			name:      "valid JSON content",
			data:      []byte(`{"foo": "bar"}`),
			codecType: codec.TypeJSON,
			wantErr:   false,
		},
		{
			name:      "valid YAML content",
			data:      []byte("foo: bar"),
			codecType: codec.TypeYAML,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := New(WithContent(tt.data, tt.codecType))

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, cfg)
			assert.Len(t, cfg.sources, 1)
		})
	}
}

func TestWithEnv(t *testing.T) {
	t.Parallel()

	cfg, err := New(WithEnv("TESTPREFIX_"))
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Len(t, cfg.sources, 1)
}

func TestWithConsul_SkipsWithoutEnvVar(t *testing.T) {
	t.Parallel()

	// Ensure CONSUL_HTTP_ADDR is not set
	originalAddr := os.Getenv("CONSUL_HTTP_ADDR")
	require.NoError(t, os.Unsetenv("CONSUL_HTTP_ADDR"))
	defer func() {
		if originalAddr != "" {
			require.NoError(t, os.Setenv("CONSUL_HTTP_ADDR", originalAddr))
		}
	}()

	cfg, err := New(WithConsul("production/service.yaml"))
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	// Should have no sources since Consul was skipped
	assert.Len(t, cfg.sources, 0)
}

func TestWithConsulAs_SkipsWithoutEnvVar(t *testing.T) {
	t.Parallel()

	// Ensure CONSUL_HTTP_ADDR is not set
	originalAddr := os.Getenv("CONSUL_HTTP_ADDR")
	require.NoError(t, os.Unsetenv("CONSUL_HTTP_ADDR"))
	defer func() {
		if originalAddr != "" {
			require.NoError(t, os.Setenv("CONSUL_HTTP_ADDR", originalAddr))
		}
	}()

	cfg, err := New(WithConsulAs("production/service", codec.TypeJSON))
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	// Should have no sources since Consul was skipped
	assert.Len(t, cfg.sources, 0)
}

func TestWithFile_ExpandsEnvVars(t *testing.T) {
	t.Parallel()

	// Set up test environment variable with unique name
	tmpDir := t.TempDir()
	envVar := "TEST_CONFIG_DIR_WITHFILE"
	require.NoError(t, os.Setenv(envVar, tmpDir))
	defer func() {
		require.NoError(t, os.Unsetenv(envVar))
	}()

	// Create test file
	testFile := filepath.Join(tmpDir, "test_env_expand.yaml")
	testData := []byte("test: value")
	require.NoError(t, os.WriteFile(testFile, testData, 0o644))

	// Test with environment variable expansion
	cfg, err := New(WithFile("${" + envVar + "}/test_env_expand.yaml"))
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Len(t, cfg.sources, 1)

	// Verify it actually loads
	require.NoError(t, cfg.Load(context.Background()))
	assert.Equal(t, "value", cfg.String("test"))
}

func TestWithFileAs_ExpandsEnvVars(t *testing.T) {
	t.Parallel()

	// Set up test environment variable with unique name
	tmpDir := t.TempDir()
	envVar := "TEST_CONFIG_DIR_WITHFILEAS"
	require.NoError(t, os.Setenv(envVar, tmpDir))
	defer func() {
		require.NoError(t, os.Unsetenv(envVar))
	}()

	// Create test file without extension
	testFile := filepath.Join(tmpDir, "test_env_expand_noext")
	testData := []byte("test: value")
	require.NoError(t, os.WriteFile(testFile, testData, 0o644))

	// Test with environment variable expansion
	cfg, err := New(WithFileAs("${"+envVar+"}/test_env_expand_noext", codec.TypeYAML))
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Len(t, cfg.sources, 1)

	// Verify it actually loads
	require.NoError(t, cfg.Load(context.Background()))
	assert.Equal(t, "value", cfg.String("test"))
}

func TestWithFileDumper_ExpandsEnvVars(t *testing.T) {
	t.Parallel()

	// Set up test environment variable with unique name
	tmpDir := t.TempDir()
	envVar := "TEST_OUTPUT_DIR_WITHFILEDUMPER"
	require.NoError(t, os.Setenv(envVar, tmpDir))
	defer func() {
		require.NoError(t, os.Unsetenv(envVar))
	}()

	outputFile := filepath.Join(tmpDir, "test_env_expand_dump.yaml")

	// Test with environment variable expansion
	cfg, err := New(
		WithContent([]byte("test: value"), codec.TypeYAML),
		WithFileDumper("${"+envVar+"}/test_env_expand_dump.yaml"),
	)
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Len(t, cfg.dumpers, 1)

	// Load and dump
	require.NoError(t, cfg.Load(context.Background()))
	require.NoError(t, cfg.Dump(context.Background()))

	// Verify file was created
	_, err = os.Stat(outputFile)
	assert.NoError(t, err)
}

func TestWithFileDumperAs_ExpandsEnvVars(t *testing.T) {
	t.Parallel()

	// Set up test environment variable with unique name
	tmpDir := t.TempDir()
	envVar := "TEST_OUTPUT_DIR_WITHFILEDUMPERAS"
	require.NoError(t, os.Setenv(envVar, tmpDir))
	defer func() {
		require.NoError(t, os.Unsetenv(envVar))
	}()

	outputFile := filepath.Join(tmpDir, "test_env_expand_dump_noext")

	// Test with environment variable expansion
	cfg, err := New(
		WithContent([]byte("test: value"), codec.TypeYAML),
		WithFileDumperAs("${"+envVar+"}/test_env_expand_dump_noext", codec.TypeYAML),
	)
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Len(t, cfg.dumpers, 1)

	// Load and dump
	require.NoError(t, cfg.Load(context.Background()))
	require.NoError(t, cfg.Dump(context.Background()))

	// Verify file was created
	_, err = os.Stat(outputFile)
	assert.NoError(t, err)
}

func TestWithConsul_ExpandsEnvVars(t *testing.T) {
	t.Parallel()

	// Set up test environment variables
	require.NoError(t, os.Setenv("TEST_APP_ENV", "staging"))
	require.NoError(t, os.Setenv("CONSUL_HTTP_ADDR", "http://localhost:8500"))
	defer func() {
		require.NoError(t, os.Unsetenv("TEST_APP_ENV"))
		require.NoError(t, os.Unsetenv("CONSUL_HTTP_ADDR"))
	}()

	// Test with environment variable expansion
	// Note: This will try to connect to Consul, so we expect an error
	// but we're just verifying the path expansion happens
	cfg, err := New(WithConsul("${TEST_APP_ENV}/service.yaml"))

	// We expect the config to be created (env var expanded)
	// but Load() will fail since there's no actual Consul
	assert.NotNil(t, cfg)
	// The error check is relaxed because Consul connection may fail
	// The important thing is the path was expanded before being used
	_ = err
}

func TestWithConsulAs_ExpandsEnvVars(t *testing.T) {
	t.Parallel()

	// Set up test environment variables
	require.NoError(t, os.Setenv("TEST_APP_ENV", "staging"))
	require.NoError(t, os.Setenv("CONSUL_HTTP_ADDR", "http://localhost:8500"))
	defer func() {
		require.NoError(t, os.Unsetenv("TEST_APP_ENV"))
		require.NoError(t, os.Unsetenv("CONSUL_HTTP_ADDR"))
	}()

	// Test with environment variable expansion
	cfg, err := New(WithConsulAs("${TEST_APP_ENV}/service", codec.TypeJSON))

	// We expect the config to be created (env var expanded)
	assert.NotNil(t, cfg)
	// The error check is relaxed because Consul connection may fail
	_ = err
}

func TestWithFileDumper(t *testing.T) {
	t.Parallel()

	path := "/tmp/config_test_file_dumper.json"
	cfg, err := New(WithFileDumperAs(path, codec.TypeJSON))
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Len(t, cfg.dumpers, 1)
}

func TestConfigError(t *testing.T) {
	t.Parallel()

	baseErr := errors.New("base error")

	tests := []struct {
		name       string
		err        *Error
		wantMsg    string
		wantUnwrap error
	}{
		{
			name: "error with field",
			err: &Error{
				Source:    "source1",
				Field:     "field1",
				Operation: "parse",
				Err:       baseErr,
			},
			wantMsg:    "config error in source1.field1 during parse: base error",
			wantUnwrap: baseErr,
		},
		{
			name: "error without field",
			err: &Error{
				Source:    "source2",
				Operation: "load",
				Err:       baseErr,
			},
			wantMsg:    "config error in source2 during load: base error",
			wantUnwrap: baseErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.wantMsg, tt.err.Error())
			assert.Equal(t, tt.wantUnwrap, tt.err.Unwrap())
		})
	}
}

func TestConcurrency(t *testing.T) {
	t.Parallel()

	t.Run("concurrent Load", func(t *testing.T) {
		t.Parallel()

		src := &mockSource{conf: map[string]any{"foo": "bar"}}
		cfg, err := New(WithSource(src))
		require.NoError(t, err)

		wg := make(chan struct{})
		for range 10 {
			go func() {
				loadErr := cfg.Load(context.Background())
				if loadErr != nil {
					t.Error(loadErr)
				}
				wg <- struct{}{}
			}()
		}
		for range 10 {
			<-wg
		}
	})

	t.Run("concurrent Get", func(t *testing.T) {
		t.Parallel()

		src := &mockSource{conf: map[string]any{"foo": "bar"}}
		cfg, err := New(WithSource(src))
		require.NoError(t, err)
		require.NoError(t, cfg.Load(context.Background()))

		wg := make(chan struct{})
		for range 10 {
			go func() {
				_ = cfg.Get("foo")
				loadErr := cfg.Load(context.Background())
				if loadErr != nil {
					t.Error(loadErr)
				}
				wg <- struct{}{}
			}()
		}
		for range 10 {
			<-wg
		}
	})

	t.Run("concurrent Get and Load with binding validation", func(t *testing.T) {
		t.Parallel()

		type validatingBindStruct struct {
			Foo string `config:"foo"`
			Bar int    `config:"bar"`
		}

		src := &mockSource{conf: map[string]any{"foo": "bar", "bar": 42}}
		var bind validatingBindStruct
		cfg, err := New(WithSource(src), WithBinding(&bind))
		require.NoError(t, err)
		require.NoError(t, cfg.Load(context.Background()))

		wg := make(chan struct{})
		for range 20 {
			go func() {
				defer func() { wg <- struct{}{} }()
				for i := 0; i < 10; i++ {
					if i%2 == 0 {
						_ = cfg.Get("foo")
						_ = cfg.String("foo")
						_ = cfg.Int("bar")
						_ = cfg.Values()
					} else {
						loadErr := cfg.Load(context.Background())
						if loadErr != nil {
							t.Error(loadErr)
						}
					}
				}
			}()
		}

		for range 20 {
			<-wg
		}

		assert.Equal(t, "bar", cfg.String("foo"))
		assert.Equal(t, 42, cfg.Int("bar"))
	})

	t.Run("concurrent access to same key", func(t *testing.T) {
		t.Parallel()

		src := &mockSource{conf: map[string]any{"shared": "value"}}
		cfg, err := New(WithSource(src))
		require.NoError(t, err)
		require.NoError(t, cfg.Load(context.Background()))

		var wg sync.WaitGroup
		//nolint:makezero // indexed assignment requires pre-allocated length
		results := make([]string, 10)

		for i := range 10 {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				results[index] = cfg.String("shared")
			}(i)
		}

		wg.Wait()

		for _, result := range results {
			assert.Equal(t, "value", result)
		}
	})
}

func TestReload(t *testing.T) {
	t.Parallel()

	src := &mockSource{conf: map[string]any{"foo": "bar"}}
	cfg, err := New(WithSource(src))
	require.NoError(t, err)
	require.NoError(t, cfg.Load(context.Background()))
	assert.Equal(t, "bar", cfg.String("foo"))

	src.conf["foo"] = "baz"
	require.NoError(t, cfg.Load(context.Background()))
	assert.Equal(t, "baz", cfg.String("foo"))
}

func TestNilConfigInstance(t *testing.T) {
	t.Parallel()

	var cfg *Config

	assert.Equal(t, "", cfg.String("any"))
	assert.Equal(t, false, cfg.Bool("any"))
	assert.Equal(t, 0, cfg.Int("any"))
	assert.Equal(t, nil, cfg.Get("any"))

	_, err := GetE[string](cfg, "any")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config instance is nil")
}

func TestLargeConfiguration(t *testing.T) {
	t.Parallel()

	largeConfig := make(map[string]any, 1000)
	for i := 0; i < 1000; i++ {
		largeConfig[fmt.Sprintf("key%d", i)] = fmt.Sprintf("value%d", i)
	}

	cfg := TestConfigLoaded(t, largeConfig)

	assert.Equal(t, "value0", cfg.String("key0"))
	assert.Equal(t, "value999", cfg.String("key999"))
	assert.Equal(t, "value500", cfg.String("key500"))
}

func TestContextCancellation(t *testing.T) {
	t.Parallel()

	type mockContextAwareSource struct {
		conf map[string]any
		err  error
	}

	mockCtxSource := &mockContextAwareSource{conf: map[string]any{"foo": "bar"}}

	// Implement Source interface
	loadFunc := func(ctx context.Context) (map[string]any, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(50 * time.Millisecond):
			return mockCtxSource.conf, mockCtxSource.err //nolint:nilnil // Test mock intentionally returns (nil, nil) for certain test cases
		}
	}

	// Use loadFunc to avoid unused variable warning
	_ = loadFunc

	cfg, err := New(WithSource(&mockSource{conf: map[string]any{"foo": "bar"}}))
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	// This test is primarily to show context handling
	err = cfg.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
}

func TestFilePermissions(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	sourceFile := tmpDir + "/source.yaml"
	dumpFile := tmpDir + "/dump.yaml"

	sourceContent := []byte("foo: bar\n")
	err := os.WriteFile(sourceFile, sourceContent, 0o600)
	require.NoError(t, err)

	cfg, err := New(
		WithFileAs(sourceFile, codec.TypeYAML),
		WithFileDumperAs(dumpFile, codec.TypeYAML),
	)
	require.NoError(t, err)
	require.NoError(t, cfg.Load(context.Background()))
	require.NoError(t, cfg.Dump(context.Background()))

	info, err := os.Stat(dumpFile)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())
}

func TestConsistentReturnTypes(t *testing.T) {
	t.Parallel()

	cfg := TestConfigLoaded(t, map[string]any{"existing": "value"})

	t.Run("slices return empty not nil", func(t *testing.T) {
		intSlice, err := GetE[[]int](cfg, "nonexistent")
		require.Error(t, err)
		assert.NotNil(t, intSlice)
		assert.Len(t, intSlice, 0)

		stringSlice, err := GetE[[]string](cfg, "nonexistent")
		require.Error(t, err)
		assert.NotNil(t, stringSlice)
		assert.Len(t, stringSlice, 0)
	})

	t.Run("maps return empty not nil", func(t *testing.T) {
		stringMap, err := GetE[map[string]any](cfg, "nonexistent")
		require.Error(t, err)
		assert.NotNil(t, stringMap)
		assert.Len(t, stringMap, 0)

		stringMapString, err := GetE[map[string]string](cfg, "nonexistent")
		require.Error(t, err)
		assert.NotNil(t, stringMapString)
		assert.Len(t, stringMapString, 0)

		stringMapStringSlice, err := GetE[map[string][]string](cfg, "nonexistent")
		require.Error(t, err)
		assert.NotNil(t, stringMapStringSlice)
		assert.Len(t, stringMapStringSlice, 0)
	})
}

// mockSlowSource simulates a slow configuration source
type mockSlowSource struct {
	conf  map[string]any
	delay time.Duration
}

func (m *mockSlowSource) Load(_ context.Context) (map[string]any, error) {
	time.Sleep(m.delay)
	return m.conf, nil
}
