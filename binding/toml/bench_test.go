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

package toml

import (
	"bytes"
	"testing"
)

// BenchmarkTOML_SmallStruct benchmarks TOML binding with a small struct.
func BenchmarkTOML_SmallStruct(b *testing.B) {
	type Config struct {
		Title   string `toml:"title"`
		Version string `toml:"version"`
		Debug   bool   `toml:"debug"`
	}

	body := []byte(`
title = "My App"
version = "1.0.0"
debug = true
`)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, _ = TOML[Config](body)
	}
}

// BenchmarkTOML_LargeStruct benchmarks TOML binding with a larger struct.
func BenchmarkTOML_LargeStruct(b *testing.B) {
	type Database struct {
		Host     string `toml:"host"`
		Port     int    `toml:"port"`
		Name     string `toml:"name"`
		User     string `toml:"user"`
		Password string `toml:"password"`
		SSLMode  string `toml:"ssl_mode"`
	}

	type Server struct {
		Host         string `toml:"host"`
		Port         int    `toml:"port"`
		ReadTimeout  string `toml:"read_timeout"`
		WriteTimeout string `toml:"write_timeout"`
	}

	type Config struct {
		Title    string   `toml:"title"`
		Version  string   `toml:"version"`
		Server   Server   `toml:"server"`
		Database Database `toml:"database"`
	}

	body := []byte(`
title = "myservice"
version = "1.0.0"

[server]
host = "0.0.0.0"
port = 8080
read_timeout = "30s"
write_timeout = "30s"

[database]
host = "localhost"
port = 5432
name = "mydb"
user = "admin"
password = "secret"
ssl_mode = "disable"
`)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, _ = TOML[Config](body)
	}
}

// BenchmarkTOML_Arrays benchmarks TOML binding with arrays.
func BenchmarkTOML_Arrays(b *testing.B) {
	type Config struct {
		Hosts []string `toml:"hosts"`
		Ports []int    `toml:"ports"`
		Tags  []string `toml:"tags"`
	}

	body := []byte(`
hosts = ["host1.example.com", "host2.example.com", "host3.example.com", "host4.example.com"]
ports = [8080, 8081, 8082, 8083]
tags = ["production", "web", "api", "v2"]
`)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, _ = TOML[Config](body)
	}
}

// BenchmarkTOML_ArrayOfTables benchmarks TOML binding with array of tables.
func BenchmarkTOML_ArrayOfTables(b *testing.B) {
	type Product struct {
		Name  string `toml:"name"`
		Price int    `toml:"price"`
	}

	type Catalog struct {
		Products []Product `toml:"products"`
	}

	body := []byte(`
[[products]]
name = "Widget"
price = 100

[[products]]
name = "Gadget"
price = 200

[[products]]
name = "Gizmo"
price = 300

[[products]]
name = "Thingamajig"
price = 400
`)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, _ = TOML[Catalog](body)
	}
}

// BenchmarkTOMLTo_NonGeneric benchmarks non-generic TOML binding.
func BenchmarkTOMLTo_NonGeneric(b *testing.B) {
	type Config struct {
		Title   string `toml:"title"`
		Version string `toml:"version"`
		Debug   bool   `toml:"debug"`
	}

	body := []byte(`
title = "My App"
version = "1.0.0"
debug = true
`)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		var config Config
		_ = TOMLTo(body, &config)
	}
}

// BenchmarkTOMLReader benchmarks TOML binding from io.Reader.
func BenchmarkTOMLReader(b *testing.B) {
	type Config struct {
		Title   string `toml:"title"`
		Version string `toml:"version"`
		Debug   bool   `toml:"debug"`
	}

	body := []byte(`
title = "My App"
version = "1.0.0"
debug = true
`)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		reader := bytes.NewReader(body)
		_, _ = TOMLReader[Config](reader)
	}
}

// BenchmarkTOMLWithMetadata benchmarks TOML binding with metadata.
func BenchmarkTOMLWithMetadata(b *testing.B) {
	type Config struct {
		Title   string `toml:"title"`
		Version string `toml:"version"`
	}

	body := []byte(`
title = "My App"
version = "1.0.0"
unknown = "ignored"
`)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, _, _ = TOMLWithMetadata[Config](body)
	}
}

// BenchmarkTOML_InlineTable benchmarks TOML binding with inline tables.
func BenchmarkTOML_InlineTable(b *testing.B) {
	type Point struct {
		X int `toml:"x"`
		Y int `toml:"y"`
	}

	type Config struct {
		Origin Point `toml:"origin"`
		Target Point `toml:"target"`
	}

	body := []byte(`
origin = { x = 10, y = 20 }
target = { x = 100, y = 200 }
`)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, _ = TOML[Config](body)
	}
}

// BenchmarkTOML_Parallel benchmarks TOML binding under concurrent load.
func BenchmarkTOML_Parallel(b *testing.B) {
	type Config struct {
		Title   string `toml:"title"`
		Version string `toml:"version"`
		Debug   bool   `toml:"debug"`
	}

	body := []byte(`
title = "My App"
version = "1.0.0"
debug = true
`)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = TOML[Config](body)
		}
	})
}
