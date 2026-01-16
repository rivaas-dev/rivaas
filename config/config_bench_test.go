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
	"fmt"
	"testing"
	"time"
)

// BenchmarkParallelLoading benchmarks parallel loading of multiple sources.
func BenchmarkParallelLoading(b *testing.B) {
	// Create multiple slow sources to demonstrate parallel loading benefits
	//nolint:makezero // indexed assignment requires pre-allocated length
	sources := make([]Source, 5)
	for i := range 5 {
		sources[i] = &mockSlowSource{
			conf:  map[string]any{fmt.Sprintf("key%d", i): fmt.Sprintf("value%d", i)},
			delay: 10 * time.Millisecond, // Simulate I/O delay
		}
	}

	var opts []Option
	for _, src := range sources {
		opts = append(opts, WithSource(src))
	}

	c, err := New(opts...)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		err = c.Load(b.Context())
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkNew benchmarks configuration creation.
func BenchmarkNew(b *testing.B) {
	src := &mockSource{conf: map[string]any{"foo": "bar"}}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		c, err := New(WithSource(src))
		if err != nil {
			b.Fatal(err)
		}
		_ = c
	}
}

// BenchmarkLoad benchmarks configuration loading.
func BenchmarkLoad(b *testing.B) {
	src := &mockSource{conf: map[string]any{"foo": "bar", "bar": 42}}
	c, err := New(WithSource(src))
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		err = c.Load(b.Context())
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLoadWithBinding benchmarks loading with struct binding.
func BenchmarkLoadWithBinding(b *testing.B) {
	src := &mockSource{conf: map[string]any{"foo": "bar", "bar": 42}}
	var bind bindStruct
	c, err := New(WithSource(src), WithBinding(&bind))
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		err = c.Load(b.Context())
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkGet benchmarks retrieving values by key.
func BenchmarkGet(b *testing.B) {
	src := &mockSource{conf: map[string]any{"foo": "bar", "bar": 42}}
	c, err := New(WithSource(src))
	if err != nil {
		b.Fatal(err)
	}
	if err = c.Load(b.Context()); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_ = c.Get("foo")
	}
}

// BenchmarkGetNested benchmarks retrieving nested values with dot notation.
func BenchmarkGetNested(b *testing.B) {
	src := &mockSource{conf: map[string]any{
		"outer": map[string]any{
			"inner": map[string]any{
				"deep": map[string]any{
					"value": 42,
				},
			},
		},
	}}
	c, err := New(WithSource(src))
	if err != nil {
		b.Fatal(err)
	}
	if err = c.Load(b.Context()); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_ = c.Get("outer.inner.deep.value")
	}
}

// BenchmarkGetString benchmarks string type conversion.
func BenchmarkGetString(b *testing.B) {
	src := &mockSource{conf: map[string]any{"foo": "bar"}}
	c, err := New(WithSource(src))
	if err != nil {
		b.Fatal(err)
	}
	if err = c.Load(b.Context()); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_ = c.String("foo")
	}
}

// BenchmarkGetInt benchmarks integer type conversion.
func BenchmarkGetInt(b *testing.B) {
	src := &mockSource{conf: map[string]any{"num": 42}}
	c, err := New(WithSource(src))
	if err != nil {
		b.Fatal(err)
	}
	if err = c.Load(b.Context()); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_ = c.Int("num")
	}
}

// BenchmarkGetStringSlice benchmarks slice type conversion.
func BenchmarkGetStringSlice(b *testing.B) {
	src := &mockSource{conf: map[string]any{"tags": []any{"a", "b", "c"}}}
	c, err := New(WithSource(src))
	if err != nil {
		b.Fatal(err)
	}
	if err = c.Load(b.Context()); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_ = c.StringSlice("tags")
	}
}

// BenchmarkConcurrentGet benchmarks concurrent read access.
func BenchmarkConcurrentGet(b *testing.B) {
	src := &mockSource{conf: map[string]any{"foo": "bar"}}
	c, err := New(WithSource(src))
	if err != nil {
		b.Fatal(err)
	}
	if err = c.Load(b.Context()); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = c.String("foo")
		}
	})
}

// BenchmarkLargeConfig benchmarks operations on large configurations.
func BenchmarkLargeConfig(b *testing.B) {
	// Create a large configuration map
	largeConfig := make(map[string]any, 1000)
	for i := range 1000 {
		largeConfig[fmt.Sprintf("key%d", i)] = fmt.Sprintf("value%d", i)
	}

	src := &mockSource{conf: largeConfig}
	c, err := New(WithSource(src))
	if err != nil {
		b.Fatal(err)
	}
	if err = c.Load(b.Context()); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_ = c.String("key500")
	}
}

// BenchmarkMultipleSources benchmarks merging multiple sources.
func BenchmarkMultipleSources(b *testing.B) {
	sources := []Source{
		&mockSource{conf: map[string]any{"key1": "value1"}},
		&mockSource{conf: map[string]any{"key2": "value2"}},
		&mockSource{conf: map[string]any{"key3": "value3"}},
	}

	var opts []Option
	for _, src := range sources {
		opts = append(opts, WithSource(src))
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		c, err := New(opts...)
		if err != nil {
			b.Fatal(err)
		}
		if err = c.Load(b.Context()); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDump benchmarks configuration dumping.
func BenchmarkDump(b *testing.B) {
	src := &mockSource{conf: map[string]any{"foo": "bar", "bar": 42}}
	dumper := &MockDumper{}
	c, err := New(WithSource(src), WithDumper(dumper))
	if err != nil {
		b.Fatal(err)
	}
	if err = c.Load(b.Context()); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		err = c.Dump(b.Context())
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkValidator benchmarks custom validation.
func BenchmarkValidator(b *testing.B) {
	src := &mockSource{conf: map[string]any{"port": 8080}}
	validator := func(cfg map[string]any) error {
		if _, ok := cfg["port"]; !ok {
			return fmt.Errorf("port is required")
		}
		return nil
	}

	c, err := New(WithSource(src), WithValidator(validator))
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		err = c.Load(b.Context())
		if err != nil {
			b.Fatal(err)
		}
	}
}
