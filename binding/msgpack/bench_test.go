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

package msgpack

import (
	"bytes"
	"testing"

	mp "github.com/vmihailenco/msgpack/v5"
)

// BenchmarkMsgPack_SmallStruct benchmarks MessagePack binding with a small struct.
func BenchmarkMsgPack_SmallStruct(b *testing.B) {
	type Config struct {
		Name  string `msgpack:"name"`
		Port  int    `msgpack:"port"`
		Debug bool   `msgpack:"debug"`
	}

	original := Config{
		Name:  "myapp",
		Port:  8080,
		Debug: true,
	}
	body, err := mp.Marshal(&original)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		MsgPack[Config](body)
	}
}

// BenchmarkMsgPack_LargeStruct benchmarks MessagePack binding with a larger struct.
func BenchmarkMsgPack_LargeStruct(b *testing.B) {
	type Database struct {
		Host     string `msgpack:"host"`
		Port     int    `msgpack:"port"`
		Name     string `msgpack:"name"`
		User     string `msgpack:"user"`
		Password string `msgpack:"password"` //nolint:gosec // G117: benchmark fixture
		SSLMode  string `msgpack:"ssl_mode"`
	}

	type Server struct {
		Host         string `msgpack:"host"`
		Port         int    `msgpack:"port"`
		ReadTimeout  string `msgpack:"read_timeout"`
		WriteTimeout string `msgpack:"write_timeout"`
	}

	type Config struct {
		App      string   `msgpack:"app"`
		Version  string   `msgpack:"version"`
		Server   Server   `msgpack:"server"`
		Database Database `msgpack:"database"`
	}

	original := Config{
		App:     "myservice",
		Version: "1.0.0",
		Server: Server{
			Host:         "0.0.0.0",
			Port:         8080,
			ReadTimeout:  "30s",
			WriteTimeout: "30s",
		},
		Database: Database{
			Host:     "localhost",
			Port:     5432,
			Name:     "mydb",
			User:     "admin",
			Password: "secret",
			SSLMode:  "disable",
		},
	}
	body, err := mp.Marshal(&original)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		MsgPack[Config](body)
	}
}

// BenchmarkMsgPack_Arrays benchmarks MessagePack binding with arrays.
func BenchmarkMsgPack_Arrays(b *testing.B) {
	type Config struct {
		Hosts []string `msgpack:"hosts"`
		Ports []int    `msgpack:"ports"`
		Tags  []string `msgpack:"tags"`
	}

	original := Config{
		Hosts: []string{
			"host1.example.com", "host2.example.com",
			"host3.example.com", "host4.example.com",
		},
		Ports: []int{8080, 8081, 8082, 8083},
		Tags:  []string{"production", "web", "api", "v2"},
	}
	body, err := mp.Marshal(&original)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		MsgPack[Config](body)
	}
}

// BenchmarkMsgPackTo_NonGeneric benchmarks non-generic MessagePack binding.
func BenchmarkMsgPackTo_NonGeneric(b *testing.B) {
	type Config struct {
		Name  string `msgpack:"name"`
		Port  int    `msgpack:"port"`
		Debug bool   `msgpack:"debug"`
	}

	original := Config{
		Name:  "myapp",
		Port:  8080,
		Debug: true,
	}
	body, err := mp.Marshal(&original)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		var config Config
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		MsgPackTo(body, &config)
	}
}

// BenchmarkMsgPackReader benchmarks MessagePack binding from io.Reader.
func BenchmarkMsgPackReader(b *testing.B) {
	type Config struct {
		Name  string `msgpack:"name"`
		Port  int    `msgpack:"port"`
		Debug bool   `msgpack:"debug"`
	}

	original := Config{
		Name:  "myapp",
		Port:  8080,
		Debug: true,
	}
	body, err := mp.Marshal(&original)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		reader := bytes.NewReader(body)
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		MsgPackReader[Config](reader)
	}
}

// BenchmarkMsgPack_WithJSONTag benchmarks MessagePack binding with JSON tags.
func BenchmarkMsgPack_WithJSONTag(b *testing.B) {
	type Config struct {
		Name  string `json:"name"`
		Port  int    `json:"port"`
		Debug bool   `json:"debug"`
	}

	buf := &bytes.Buffer{}
	enc := mp.NewEncoder(buf)
	enc.SetCustomStructTag("json")
	err := enc.Encode(&Config{
		Name:  "myapp",
		Port:  8080,
		Debug: true,
	})
	if err != nil {
		b.Fatal(err)
	}
	body := buf.Bytes()

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		MsgPack[Config](body, WithJSONTag())
	}
}

// BenchmarkMsgPack_Maps benchmarks MessagePack binding with maps.
func BenchmarkMsgPack_Maps(b *testing.B) {
	type Config struct {
		Settings map[string]string `msgpack:"settings"`
		Metadata map[string]int    `msgpack:"metadata"`
	}

	original := Config{
		Settings: map[string]string{
			"log_level":   "debug",
			"environment": "production",
			"region":      "us-east-1",
		},
		Metadata: map[string]int{
			"version":  1,
			"priority": 10,
			"replicas": 3,
		},
	}
	body, err := mp.Marshal(&original)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		MsgPack[Config](body)
	}
}

// BenchmarkMsgPack_DisallowUnknown benchmarks MessagePack with unknown field checking.
func BenchmarkMsgPack_DisallowUnknown(b *testing.B) {
	type Config struct {
		Name  string `msgpack:"name"`
		Port  int    `msgpack:"port"`
		Debug bool   `msgpack:"debug"`
	}

	original := Config{
		Name:  "myapp",
		Port:  8080,
		Debug: true,
	}
	body, err := mp.Marshal(&original)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		//nolint:errcheck // Benchmark measures performance; error checking would skew results
		MsgPack[Config](body, WithDisallowUnknown())
	}
}

// BenchmarkMsgPack_Parallel benchmarks MessagePack binding under concurrent load.
func BenchmarkMsgPack_Parallel(b *testing.B) {
	type Config struct {
		Name  string `msgpack:"name"`
		Port  int    `msgpack:"port"`
		Debug bool   `msgpack:"debug"`
	}

	original := Config{
		Name:  "myapp",
		Port:  8080,
		Debug: true,
	}
	body, err := mp.Marshal(&original)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			//nolint:errcheck // Benchmark measures performance; error checking would skew results
			MsgPack[Config](body)
		}
	})
}
