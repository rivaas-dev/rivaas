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

//go:build !integration

package proto

import (
	"bytes"
	"testing"

	"google.golang.org/protobuf/proto"

	"rivaas.dev/binding/proto/testdata"
)

// BenchmarkProto_SmallMessage benchmarks Protocol Buffers binding with a small message.
func BenchmarkProto_SmallMessage(b *testing.B) {
	user := &testdata.User{
		Name:   "John",
		Email:  "john@example.com",
		Age:    30,
		Active: true,
	}
	body, err := proto.Marshal(user)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		Proto[*testdata.User](body)
	}
}

// BenchmarkProto_LargeMessage benchmarks Protocol Buffers binding with a larger message.
func BenchmarkProto_LargeMessage(b *testing.B) {
	config := &testdata.Config{
		Title: "Production Config with a longer title for testing",
		Server: &testdata.Server{
			Host: "production-server.example.com",
			Port: 8080,
		},
		Database: &testdata.Database{
			Host:     "database-server.example.com",
			Port:     5432,
			Name:     "production_database",
			User:     "admin_user",
			Password: "super_secret_password_123",
		},
	}
	body, err := proto.Marshal(config)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		Proto[*testdata.Config](body)
	}
}

// BenchmarkProto_RepeatedFields benchmarks Protocol Buffers binding with repeated fields.
func BenchmarkProto_RepeatedFields(b *testing.B) {
	product := &testdata.Product{
		Name:   "Widget Pro",
		Tags:   []string{"electronics", "gadget", "sale", "featured", "new"},
		Prices: []int32{100, 150, 200, 250, 300},
	}
	body, err := proto.Marshal(product)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		Proto[*testdata.Product](body)
	}
}

// BenchmarkProto_MapFields benchmarks Protocol Buffers binding with map fields.
func BenchmarkProto_MapFields(b *testing.B) {
	settings := &testdata.Settings{
		Name: "AppSettings",
		Metadata: map[string]string{
			"version":     "1.0.0",
			"environment": "production",
			"region":      "us-east-1",
			"tier":        "premium",
		},
	}
	body, err := proto.Marshal(settings)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		Proto[*testdata.Settings](body)
	}
}

// BenchmarkProtoTo_NonGeneric benchmarks non-generic Protocol Buffers binding.
func BenchmarkProtoTo_NonGeneric(b *testing.B) {
	user := &testdata.User{
		Name:   "John",
		Email:  "john@example.com",
		Age:    30,
		Active: true,
	}
	body, err := proto.Marshal(user)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		var result testdata.User
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		ProtoTo(body, &result)
	}
}

// BenchmarkProtoReader benchmarks Protocol Buffers binding from io.Reader.
func BenchmarkProtoReader(b *testing.B) {
	user := &testdata.User{
		Name:   "John",
		Email:  "john@example.com",
		Age:    30,
		Active: true,
	}
	body, err := proto.Marshal(user)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		reader := bytes.NewReader(body)
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		ProtoReader[*testdata.User](reader)
	}
}

// BenchmarkProto_WithOptions benchmarks Protocol Buffers binding with options.
func BenchmarkProto_WithOptions(b *testing.B) {
	user := &testdata.User{
		Name:   "John",
		Email:  "john@example.com",
		Age:    30,
		Active: true,
	}
	body, err := proto.Marshal(user)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		Proto[*testdata.User](body,
			WithDiscardUnknown(),
			WithAllowPartial(),
			WithRecursionLimit(5000),
		)
	}
}

// BenchmarkProto_Parallel benchmarks Protocol Buffers binding under concurrent load.
func BenchmarkProto_Parallel(b *testing.B) {
	user := &testdata.User{
		Name:   "John",
		Email:  "john@example.com",
		Age:    30,
		Active: true,
	}
	body, err := proto.Marshal(user)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			//nolint:errcheck // Benchmark measures performance; error checking would skew results
			Proto[*testdata.User](body)
		}
	})
}

// BenchmarkProto_LargeRepeatedFields benchmarks with larger repeated fields.
func BenchmarkProto_LargeRepeatedFields(b *testing.B) {
	tags := make([]string, 0, 100)
	prices := make([]int32, 0, 100)
	for i := range 100 {
		tags = append(tags, "tag")
		//nolint:gosec // G115: Safe conversion - i is in range [0,99], i*10 max is 990
		prices = append(prices, int32(i*10))
	}

	product := &testdata.Product{
		Name:   "Widget Pro",
		Tags:   tags,
		Prices: prices,
	}
	body, err := proto.Marshal(product)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		Proto[*testdata.Product](body)
	}
}

// BenchmarkProto_LargeMapFields benchmarks with larger map fields.
func BenchmarkProto_LargeMapFields(b *testing.B) {
	metadata := make(map[string]string, 50)
	for range 50 {
		metadata["key"] = "value"
	}

	settings := &testdata.Settings{
		Name:     "AppSettings",
		Metadata: metadata,
	}
	body, err := proto.Marshal(settings)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		Proto[*testdata.Settings](body)
	}
}
