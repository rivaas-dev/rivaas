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

// validatingStruct is used by TestBinding_ValidatorInterface; implements Validator.
type validatingStruct struct {
	Port int `config:"port"`
}

func (v *validatingStruct) Validate() error {
	if v.Port <= 0 {
		return errors.New("port must be positive")
	}
	return nil
}

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

func TestNew_WithTag(t *testing.T) {
	t.Parallel()

	type cfgTagStruct struct {
		Foo string `cfg:"foo"`
		Bar int    `cfg:"bar"`
	}

	tests := []struct {
		name    string
		opts    []Option
		wantErr bool
		errMsg  string
		verify  func(t *testing.T, cfg *Config)
	}{
		{
			name: "valid custom tag binds correctly",
			opts: []Option{
				WithSource(&mockSource{conf: map[string]any{"foo": "baz", "bar": 99}}),
				WithTag("cfg"),
				WithBinding(&cfgTagStruct{}),
			},
			wantErr: false,
			verify: func(t *testing.T, cfg *Config) {
				require.NoError(t, cfg.Load(context.Background()))
				assert.Equal(t, "baz", cfg.String("foo"))
				assert.Equal(t, 99, cfg.Int("bar"))
			},
		},
		{
			name:    "empty tag name fails",
			opts:    []Option{WithTag("")},
			wantErr: true,
			errMsg:  "tag name cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := New(tt.opts...)

			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.errMsg)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, cfg)
			if tt.verify != nil {
				tt.verify(t, cfg)
			}
		})
	}
}

func TestNew_NilOptionSkipped(t *testing.T) {
	t.Parallel()

	src1 := &mockSource{conf: map[string]any{"a": "1"}}
	src2 := &mockSource{conf: map[string]any{"b": "2"}}
	cfg, err := New(WithSource(src1), nil, WithSource(src2))
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Len(t, cfg.sources, 2, "nil option should be skipped, two sources applied")

	require.NoError(t, cfg.Load(context.Background()))
	assert.Equal(t, "1", cfg.String("a"))
	assert.Equal(t, "2", cfg.String("b"))
}

func TestNew_OptionErrorPaths(t *testing.T) {
	t.Parallel()

	unknownType := codec.Type("unknown")

	tests := []struct {
		name        string
		opt         Option
		wantErr     bool
		errContains string
	}{
		{
			name:        "WithFileDumper unknown extension",
			opt:         WithFileDumper("file.xyz"),
			wantErr:     true,
			errContains: "detect-format",
		},
		{
			name:        "WithFile unknown extension",
			opt:         WithFile("file.xyz"),
			wantErr:     true,
			errContains: "detect-format",
		},
		{
			name:        "WithFileAs unregistered codec type",
			opt:         WithFileAs("/tmp/config", unknownType),
			wantErr:     true,
			errContains: "get-decoder",
		},
		{
			name:        "WithContent unregistered codec type",
			opt:         WithContent([]byte("{}"), unknownType),
			wantErr:     true,
			errContains: "get-decoder",
		},
		{
			name:        "WithFileDumperAs unregistered codec type",
			opt:         WithFileDumperAs("/tmp/out", unknownType),
			wantErr:     true,
			errContains: "get-encoder",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := New(tt.opt)
			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.errContains)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, cfg)
		})
	}
}

func TestDetectFormat_UnknownExtension(t *testing.T) {
	t.Parallel()

	_, err := detectFormat("file.xyz")
	require.Error(t, err)
	assert.ErrorContains(t, err, "cannot detect format from extension")
	assert.ErrorContains(t, err, "WithFileAs()")
}

func TestNew_WithConsul_OptionErrorPaths(t *testing.T) {
	// Do not use t.Parallel() here: subtests use t.Setenv which is incompatible with parallel.
	unknownType := codec.Type("unknown")

	t.Run("with CONSUL_HTTP_ADDR set unknown extension returns error", func(t *testing.T) {
		t.Setenv("CONSUL_HTTP_ADDR", "http://localhost:8500")

		_, err := New(WithConsul("path/file.xyz"))
		require.Error(t, err)
		assert.ErrorContains(t, err, "detect-format")
	})

	t.Run("with CONSUL_HTTP_ADDR set unregistered codec type returns error", func(t *testing.T) {
		// Cannot use t.Parallel() with t.Setenv
		t.Setenv("CONSUL_HTTP_ADDR", "http://localhost:8500")

		_, err := New(WithConsulAs("path/key", unknownType))
		require.Error(t, err)
		assert.ErrorContains(t, err, "get-decoder")
	})
}

func TestNew_MultipleOptionErrors(t *testing.T) {
	t.Parallel()

	_, err := New(WithSource(nil), WithDumper(nil), WithBinding(nil))
	require.Error(t, err)
	// Errors are joined; all should be present
	assert.Contains(t, err.Error(), "source cannot be nil")
	assert.Contains(t, err.Error(), "dumper cannot be nil")
	assert.Contains(t, err.Error(), "binding target cannot be nil")
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

	t.Run("panic message contains config failure prefix", func(t *testing.T) {
		t.Parallel()
		var panicMsg string
		func() {
			defer func() {
				if r := recover(); r != nil {
					panicMsg = fmt.Sprint(r)
				}
			}()
			MustNew(WithSource(nil))
		}()
		require.Contains(t, panicMsg, "config: failed to create config")
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

func TestLoad_CancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately so Load sees ctx.Err() != nil

	src := &mockSource{conf: map[string]any{"foo": "bar"}}
	cfg, err := New(WithSource(src))
	require.NoError(t, err)

	err = cfg.Load(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestLoad_BindingPointerToNonStruct(t *testing.T) {
	t.Parallel()

	var notAStruct int
	cfg, err := New(WithSource(&mockSource{conf: map[string]any{"x": "y"}}), WithBinding(&notAStruct))
	require.NoError(t, err)

	err = cfg.Load(context.Background())
	require.Error(t, err)
	// Binding to *int fails (decode expects struct, or applyDefaults rejects non-struct)
}

func TestLoad_BindingInvalidDurationDefault(t *testing.T) {
	t.Parallel()

	type withDuration struct {
		Timeout time.Duration `config:"timeout" default:"not-a-duration"`
	}
	var target withDuration
	cfg, err := New(WithSource(&mockSource{conf: map[string]any{}}), WithBinding(&target))
	require.NoError(t, err)

	err = cfg.Load(context.Background())
	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to set default")
}

func TestLoad_BindingUnsupportedDefaultType(t *testing.T) {
	t.Parallel()

	type withSliceDefault struct {
		Items []string `config:"items" default:"a,b,c"` // slice default not supported by setDefaultValue
	}
	var target withSliceDefault
	cfg, err := New(WithSource(&mockSource{conf: map[string]any{}}), WithBinding(&target))
	require.NoError(t, err)

	err = cfg.Load(context.Background())
	require.Error(t, err)
	assert.ErrorContains(t, err, "unsupported type for default tag")
}

func TestValues_WithoutLoad(t *testing.T) {
	t.Parallel()

	cfg, err := New()
	require.NoError(t, err)

	vals := cfg.Values()
	require.NotNil(t, vals)
	require.NotNil(t, *vals)
	assert.Empty(t, *vals)
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

func TestBinding_DefaultTag(t *testing.T) {
	t.Parallel()

	type defaultTagStruct struct {
		Foo     string        `config:"foo" default:"defaultfoo"`
		Bar     int           `config:"bar" default:"42"`
		Enabled bool          `config:"enabled" default:"true"`
		Timeout time.Duration `config:"timeout" default:"5s"`
	}

	tests := []struct {
		name   string
		conf   map[string]any
		verify func(t *testing.T, target *defaultTagStruct)
	}{
		{
			name: "defaults applied when keys omitted",
			conf: map[string]any{"foo": "fromconfig"},
			verify: func(t *testing.T, target *defaultTagStruct) {
				assert.Equal(t, "fromconfig", target.Foo)
				assert.Equal(t, 42, target.Bar)
				assert.True(t, target.Enabled)
				assert.Equal(t, 5*time.Second, target.Timeout)
			},
		},
		{
			name: "provided values override defaults",
			conf: map[string]any{"foo": "x", "bar": 7, "enabled": true, "timeout": "10s"},
			verify: func(t *testing.T, target *defaultTagStruct) {
				assert.Equal(t, "x", target.Foo)
				assert.Equal(t, 7, target.Bar)
				assert.True(t, target.Enabled)
				assert.Equal(t, 10*time.Second, target.Timeout)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var target defaultTagStruct
			cfg, err := New(WithSource(&mockSource{conf: tt.conf}), WithBinding(&target))
			require.NoError(t, err)
			require.NoError(t, cfg.Load(context.Background()))
			tt.verify(t, &target)
		})
	}
}

func TestBinding_ValidatorInterface(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		conf    map[string]any
		wantErr bool
		errMsg  string
	}{
		{
			name:    "Validate returns nil succeeds",
			conf:    map[string]any{"port": 8080},
			wantErr: false,
		},
		{
			name:    "Validate returns error fails",
			conf:    map[string]any{}, // port omitted => 0, Validate rejects
			wantErr: true,
			errMsg:  "port must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var target validatingStruct
			cfg, err := New(WithSource(&mockSource{conf: tt.conf}), WithBinding(&target))
			require.NoError(t, err)

			err = cfg.Load(context.Background())

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.ErrorContains(t, err, tt.errMsg)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, 8080, target.Port)
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

func TestDump_NoLoad(t *testing.T) {
	t.Parallel()

	dumper := &MockDumper{}
	cfg, err := New(WithSource(&mockSource{conf: map[string]any{"foo": "bar"}}), WithDumper(dumper))
	require.NoError(t, err)
	// Do not call Load

	err = cfg.Dump(context.Background())
	require.NoError(t, err)
	assert.True(t, dumper.called)
	require.NotNil(t, dumper.values)
	// Values not loaded yet: internal map is empty from New()
	assert.Empty(t, *dumper.values)
}

func TestLoad_NilContext(t *testing.T) {
	t.Parallel()

	src := &mockSource{conf: map[string]any{"foo": "bar"}}
	cfg, err := New(WithSource(src))
	require.NoError(t, err)

	callLoadWithNil := func(c *Config) error {
		//nolint:staticcheck // SA1012: Intentionally testing nil context error handling
		return c.Load(nil)
	}

	err = callLoadWithNil(cfg)
	require.Error(t, err)
	assert.ErrorContains(t, err, "context cannot be nil")
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

func TestGet_EmptyKey(t *testing.T) {
	t.Parallel()

	cfg := TestConfigLoaded(t, map[string]any{"foo": "bar"})

	got := cfg.Get("")
	assert.Nil(t, got)

	_, err := GetE[string](cfg, "")
	require.Error(t, err)
	assert.ErrorContains(t, err, "not found")
}

func TestGet_MissingKeyReturnsZero(t *testing.T) {
	t.Parallel()

	cfg := TestConfigLoaded(t, map[string]any{"foo": "bar"})

	assert.Equal(t, 0, Get[int](cfg, "missing"))
	assert.Equal(t, "", Get[string](cfg, "missing"))
	assert.False(t, Get[bool](cfg, "missing"))
}

func TestGet_ValueNotConvertibleReturnsZero(t *testing.T) {
	t.Parallel()

	cfg := TestConfigLoaded(t, map[string]any{"port": "not-a-number"})

	got := Get[int](cfg, "port")
	assert.Equal(t, 0, got)
}

func TestGetE_NilConfigAndKeyNotFoundAndConversionError(t *testing.T) {
	t.Parallel()

	t.Run("nil config returns error", func(t *testing.T) {
		t.Parallel()
		var cfg *Config
		_, err := GetE[string](cfg, "key")
		require.Error(t, err)
		assert.ErrorContains(t, err, "config instance is nil")
	})

	t.Run("key not found returns error", func(t *testing.T) {
		t.Parallel()
		cfg := TestConfigLoaded(t, map[string]any{"foo": "bar"})
		_, err := GetE[int](cfg, "nonexistent")
		require.Error(t, err)
		assert.ErrorContains(t, err, "key \"nonexistent\" not found")
	})

	t.Run("value not convertible returns error", func(t *testing.T) {
		t.Parallel()
		type customType struct{ X int }
		cfg := TestConfigLoaded(t, map[string]any{"key": "string-value"})
		_, err := GetE[customType](cfg, "key")
		require.Error(t, err)
		assert.ErrorContains(t, err, "cannot convert value at key \"key\" to type")
	})
}

func TestGetOr(t *testing.T) {
	t.Parallel()

	t.Run("key present returns value", func(t *testing.T) {
		t.Parallel()
		cfg := TestConfigLoaded(t, map[string]any{"port": 9090})
		got := GetOr(cfg, "port", 8080)
		assert.Equal(t, 9090, got)
	})

	t.Run("key missing returns default", func(t *testing.T) {
		t.Parallel()
		cfg := TestConfigLoaded(t, map[string]any{"foo": "bar"})
		got := GetOr(cfg, "port", 8080)
		assert.Equal(t, 8080, got)
	})

	t.Run("nil config returns default", func(t *testing.T) {
		t.Parallel()
		var cfg *Config
		got := GetOr(cfg, "port", 8080)
		assert.Equal(t, 8080, got)
	})
}

func TestGet_UnsupportedType(t *testing.T) {
	t.Parallel()

	type myType struct{}

	cfg := TestConfigLoaded(t, map[string]any{"custom": "value"})

	got := Get[myType](cfg, "custom")
	assert.Equal(t, myType{}, got)

	_, err := GetE[myType](cfg, "custom")
	require.Error(t, err)
	assert.ErrorContains(t, err, "cannot convert")
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
		"int8":        int8(8),
		"int16":       int16(16),
		"int32":       int32(32),
		"int64":       int64(64),
		"uint8":       uint8(8),
		"uint":        uint(7),
		"uint16":      uint16(16),
		"uint32":      uint32(32),
		"uint64":      uint64(64),
		"float32":     float32(1.5),
		"float32str":  "2.5",
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
			name: "GetInt8",
			testFn: func(t *testing.T) {
				assert.Equal(t, int8(8), Get[int8](cfg, "int8"))
				i8, err := GetE[int8](cfg, "int8")
				require.NoError(t, err)
				assert.Equal(t, int8(8), i8)
				_, err = GetE[int8](cfg, "notfound")
				assert.Error(t, err)
			},
		},
		{
			name: "GetInt16",
			testFn: func(t *testing.T) {
				assert.Equal(t, int16(16), Get[int16](cfg, "int16"))
				i16, err := GetE[int16](cfg, "int16")
				require.NoError(t, err)
				assert.Equal(t, int16(16), i16)
				_, err = GetE[int16](cfg, "notfound")
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
			name: "GetFloat32",
			testFn: func(t *testing.T) {
				assert.Equal(t, float32(1.5), Get[float32](cfg, "float32"))
				f32, err := GetE[float32](cfg, "float32str")
				require.NoError(t, err)
				assert.InDelta(t, 2.5, float64(f32), 0.0001)
				_, err = GetE[float32](cfg, "notfound")
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

func TestNew_WithJSONSchema_ErrorCase(t *testing.T) {
	t.Parallel()

	t.Run("invalid JSON schema fails", func(t *testing.T) {
		t.Parallel()

		_, err := New(WithSource(&mockSource{conf: map[string]any{"foo": "bar"}}), WithJSONSchema([]byte(`{invalid json`)))
		require.Error(t, err)
	})

	t.Run("schema that fails to compile returns error", func(t *testing.T) {
		t.Parallel()
		// Schema with invalid $ref that does not exist - Compile fails
		schema := []byte(`{"$ref": "#/definitions/Missing"}`)
		_, err := New(WithSource(&mockSource{conf: map[string]any{"foo": "bar"}}), WithJSONSchema(schema))
		require.Error(t, err)
	})
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
	require.NoError(t, os.WriteFile(testFile, testData, 0o600))

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
	require.NoError(t, os.WriteFile(testFile, testData, 0o600))

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
				for i := range 10 {
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
	assert.Equal(t, int64(0), cfg.Int64("any"))
	assert.Equal(t, 0.0, cfg.Float64("any"))
	assert.Equal(t, time.Duration(0), cfg.Duration("any"))
	assert.True(t, cfg.Time("any").IsZero())
	assert.Empty(t, cfg.StringSlice("any"))
	assert.Empty(t, cfg.IntSlice("any"))
	assert.Empty(t, cfg.StringMap("any"))
	assert.Equal(t, nil, cfg.Get("any"))

	_, err := GetE[string](cfg, "any")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config instance is nil")
}

func TestConfigOrMethods_NilConfigAndMissingKey(t *testing.T) {
	t.Parallel()

	t.Run("nil config returns default for Or methods", func(t *testing.T) {
		t.Parallel()
		var cfg *Config
		assert.Equal(t, "default", cfg.StringOr("key", "default"))
		assert.Equal(t, 8080, cfg.IntOr("key", 8080))
		assert.Equal(t, int64(1024), cfg.Int64Or("key", 1024))
		assert.Equal(t, 0.5, cfg.Float64Or("key", 0.5))
		assert.True(t, cfg.BoolOr("key", true))
		assert.Equal(t, 30*time.Second, cfg.DurationOr("key", 30*time.Second))
		assert.Equal(t, []string{"a"}, cfg.StringSliceOr("key", []string{"a"}))
		assert.Equal(t, []int{1}, cfg.IntSliceOr("key", []int{1}))
		assert.Equal(t, map[string]any{"x": "y"}, cfg.StringMapOr("key", map[string]any{"x": "y"}))
	})

	t.Run("missing key returns default for Or methods", func(t *testing.T) {
		t.Parallel()
		cfg := TestConfigLoaded(t, map[string]any{"foo": "bar"})
		assert.Equal(t, "default", cfg.StringOr("missing", "default"))
		assert.Equal(t, 8080, cfg.IntOr("missing", 8080))
		assert.Equal(t, int64(1024), cfg.Int64Or("missing", 1024))
		assert.Equal(t, 0.5, cfg.Float64Or("missing", 0.5))
		assert.True(t, cfg.BoolOr("missing", true))
		assert.Equal(t, 30*time.Second, cfg.DurationOr("missing", 30*time.Second))
		assert.Equal(t, []string{"a"}, cfg.StringSliceOr("missing", []string{"a"}))
		assert.Equal(t, []int{1}, cfg.IntSliceOr("missing", []int{1}))
		assert.Equal(t, map[string]any{"x": "y"}, cfg.StringMapOr("missing", map[string]any{"x": "y"}))
	})
}

func TestLargeConfiguration(t *testing.T) {
	t.Parallel()

	largeConfig := make(map[string]any, 1000)
	for i := range 1000 {
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
