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

package yaml_test

import (
	"bytes"
	"fmt"

	"rivaas.dev/binding/yaml"
)

// ExampleYAML demonstrates basic YAML binding.
func ExampleYAML() {
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

	config, err := yaml.YAML[Config](body)
	if err != nil {
		_, _ = fmt.Printf("Error: %v\n", err)
		return
	}

	_, _ = fmt.Printf("Name: %s, Port: %d, Debug: %v\n", config.Name, config.Port, config.Debug)
	// Output: Name: myapp, Port: 8080, Debug: true
}

// ExampleYAMLTo demonstrates non-generic YAML binding.
func ExampleYAMLTo() {
	type Server struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	}

	body := []byte(`
host: localhost
port: 3000
`)

	var server Server
	err := yaml.YAMLTo(body, &server)
	if err != nil {
		_, _ = fmt.Printf("Error: %v\n", err)
		return
	}

	_, _ = fmt.Printf("Host: %s, Port: %d\n", server.Host, server.Port)
	// Output: Host: localhost, Port: 3000
}

// ExampleYAMLReader demonstrates binding from an io.Reader.
func ExampleYAMLReader() {
	type Database struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		Database string `yaml:"database"`
	}

	body := bytes.NewReader([]byte(`
host: db.example.com
port: 5432
database: mydb
`))

	db, err := yaml.YAMLReader[Database](body)
	if err != nil {
		_, _ = fmt.Printf("Error: %v\n", err)
		return
	}

	_, _ = fmt.Printf("Database: %s@%s:%d\n", db.Database, db.Host, db.Port)
	// Output: Database: mydb@db.example.com:5432
}

// ExampleYAML_withStrict demonstrates strict YAML parsing.
func ExampleYAML_withStrict() {
	type Config struct {
		Name string `yaml:"name"`
	}

	// YAML with only known fields
	body := []byte(`name: myapp`)

	config, err := yaml.YAML[Config](body, yaml.WithStrict())
	if err != nil {
		_, _ = fmt.Printf("Error: %v\n", err)
		return
	}

	_, _ = fmt.Printf("Name: %s\n", config.Name)
	// Output: Name: myapp
}

// ExampleYAML_nestedStructs demonstrates binding nested YAML structures.
func ExampleYAML_nestedStructs() {
	type Database struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	}

	type Server struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	}

	type Config struct {
		App      string   `yaml:"app"`
		Server   Server   `yaml:"server"`
		Database Database `yaml:"database"`
	}

	body := []byte(`
app: myservice
server:
  host: 0.0.0.0
  port: 8080
database:
  host: localhost
  port: 5432
`)

	config, err := yaml.YAML[Config](body)
	if err != nil {
		_, _ = fmt.Printf("Error: %v\n", err)
		return
	}

	_, _ = fmt.Printf("App: %s, Server: %s:%d, DB: %s:%d\n",
		config.App,
		config.Server.Host, config.Server.Port,
		config.Database.Host, config.Database.Port)
	// Output: App: myservice, Server: 0.0.0.0:8080, DB: localhost:5432
}

// ExampleYAML_arrays demonstrates binding YAML arrays.
func ExampleYAML_arrays() {
	type Config struct {
		Hosts []string `yaml:"hosts"`
		Ports []int    `yaml:"ports"`
	}

	body := []byte(`
hosts:
  - host1.example.com
  - host2.example.com
ports:
  - 8080
  - 8081
`)

	config, err := yaml.YAML[Config](body)
	if err != nil {
		_, _ = fmt.Printf("Error: %v\n", err)
		return
	}

	_, _ = fmt.Printf("Hosts: %v, Ports: %v\n", config.Hosts, config.Ports)
	// Output: Hosts: [host1.example.com host2.example.com], Ports: [8080 8081]
}

// ExampleYAML_maps demonstrates binding YAML maps.
func ExampleYAML_maps() {
	type Config struct {
		Settings map[string]string `yaml:"settings"`
	}

	body := []byte(`
settings:
  log_level: debug
  environment: production
  region: us-east-1
`)

	config, err := yaml.YAML[Config](body)
	if err != nil {
		_, _ = fmt.Printf("Error: %v\n", err)
		return
	}

	_, _ = fmt.Printf("Log Level: %s\n", config.Settings["log_level"])
	// Output: Log Level: debug
}
