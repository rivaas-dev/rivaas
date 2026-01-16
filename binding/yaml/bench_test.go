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

package yaml

import (
	"bytes"
	"testing"
)

// BenchmarkYAML_SmallStruct benchmarks YAML binding with a small struct.
func BenchmarkYAML_SmallStruct(b *testing.B) {
	type Config struct {
		Name  string `yaml:"name"`
		Port  int    `yaml:"port"`
		Debug bool   `yaml:"debug"`
	}

	body := []byte(`
name: myapp
port: 8080
debug: true
`)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		YAML[Config](body)
	}
}

// BenchmarkYAML_LargeStruct benchmarks YAML binding with a larger struct.
func BenchmarkYAML_LargeStruct(b *testing.B) {
	type Database struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		Name     string `yaml:"name"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		SSLMode  string `yaml:"ssl_mode"` //nolint:tagliatelle // snake_case is standard YAML convention
	}

	type Server struct {
		Host         string `yaml:"host"`
		Port         int    `yaml:"port"`
		ReadTimeout  string `yaml:"read_timeout"`  //nolint:tagliatelle // snake_case is standard YAML convention
		WriteTimeout string `yaml:"write_timeout"` //nolint:tagliatelle // snake_case is standard YAML convention
		IdleTimeout  string `yaml:"idle_timeout"`  //nolint:tagliatelle // snake_case is standard YAML convention
	}

	type Logging struct {
		Level  string `yaml:"level"`
		Format string `yaml:"format"`
		Output string `yaml:"output"`
	}

	type Config struct {
		App      string   `yaml:"app"`
		Version  string   `yaml:"version"`
		Server   Server   `yaml:"server"`
		Database Database `yaml:"database"`
		Logging  Logging  `yaml:"logging"`
	}

	body := []byte(`
app: myservice
version: 1.0.0
server:
  host: 0.0.0.0
  port: 8080
  read_timeout: 30s
  write_timeout: 30s
  idle_timeout: 60s
database:
  host: localhost
  port: 5432
  name: mydb
  user: admin
  password: secret
  ssl_mode: disable
logging:
  level: debug
  format: json
  output: stdout
`)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		YAML[Config](body)
	}
}

// BenchmarkYAML_Arrays benchmarks YAML binding with arrays.
func BenchmarkYAML_Arrays(b *testing.B) {
	type Config struct {
		Hosts []string `yaml:"hosts"`
		Ports []int    `yaml:"ports"`
		Tags  []string `yaml:"tags"`
	}

	body := []byte(`
hosts:
  - host1.example.com
  - host2.example.com
  - host3.example.com
  - host4.example.com
  - host5.example.com
ports:
  - 8080
  - 8081
  - 8082
  - 8083
  - 8084
tags:
  - production
  - web
  - api
  - v2
`)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		YAML[Config](body)
	}
}

// BenchmarkYAMLTo_NonGeneric benchmarks non-generic YAML binding.
func BenchmarkYAMLTo_NonGeneric(b *testing.B) {
	type Config struct {
		Name  string `yaml:"name"`
		Port  int    `yaml:"port"`
		Debug bool   `yaml:"debug"`
	}

	body := []byte(`
name: myapp
port: 8080
debug: true
`)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		var config Config
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		YAMLTo(body, &config)
	}
}

// BenchmarkYAMLReader benchmarks YAML binding from io.Reader.
func BenchmarkYAMLReader(b *testing.B) {
	type Config struct {
		Name  string `yaml:"name"`
		Port  int    `yaml:"port"`
		Debug bool   `yaml:"debug"`
	}

	body := []byte(`
name: myapp
port: 8080
debug: true
`)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		reader := bytes.NewReader(body)
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		YAMLReader[Config](reader)
	}
}

// BenchmarkYAML_Strict benchmarks YAML binding with strict mode.
func BenchmarkYAML_Strict(b *testing.B) {
	type Config struct {
		Name  string `yaml:"name"`
		Port  int    `yaml:"port"`
		Debug bool   `yaml:"debug"`
	}

	body := []byte(`
name: myapp
port: 8080
debug: true
`)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		YAML[Config](body, WithStrict())
	}
}

// BenchmarkYAML_Maps benchmarks YAML binding with maps.
func BenchmarkYAML_Maps(b *testing.B) {
	type Config struct {
		Settings map[string]string `yaml:"settings"`
		Metadata map[string]int    `yaml:"metadata"`
	}

	body := []byte(`
settings:
  log_level: debug
  environment: production
  region: us-east-1
  service: api
metadata:
  version: 1
  priority: 10
  replicas: 3
`)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		YAML[Config](body)
	}
}

// BenchmarkYAML_Parallel benchmarks YAML binding under concurrent load.
func BenchmarkYAML_Parallel(b *testing.B) {
	type Config struct {
		Name  string `yaml:"name"`
		Port  int    `yaml:"port"`
		Debug bool   `yaml:"debug"`
	}

	body := []byte(`
name: myapp
port: 8080
debug: true
`)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			//nolint:errcheck // Benchmark measures performance; error checking would skew results
			YAML[Config](body)
		}
	})
}
