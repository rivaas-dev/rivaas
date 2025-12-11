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

package toml_test

import (
	"bytes"
	"fmt"

	"rivaas.dev/binding/toml"
)

// ExampleTOML demonstrates basic TOML binding.
func ExampleTOML() {
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

	config, err := toml.TOML[Config](body)
	if err != nil {
		_, _ = fmt.Printf("Error: %v\n", err)
		return
	}

	_, _ = fmt.Printf("Title: %s, Version: %s, Debug: %v\n", config.Title, config.Version, config.Debug)
	// Output: Title: My App, Version: 1.0.0, Debug: true
}

// ExampleTOMLTo demonstrates non-generic TOML binding.
func ExampleTOMLTo() {
	type Server struct {
		Host string `toml:"host"`
		Port int    `toml:"port"`
	}

	body := []byte(`
host = "localhost"
port = 8080
`)

	var server Server
	err := toml.TOMLTo(body, &server)
	if err != nil {
		_, _ = fmt.Printf("Error: %v\n", err)
		return
	}

	_, _ = fmt.Printf("Server: %s:%d\n", server.Host, server.Port)
	// Output: Server: localhost:8080
}

// ExampleTOMLReader demonstrates binding from an io.Reader.
func ExampleTOMLReader() {
	type Database struct {
		Host     string `toml:"host"`
		Port     int    `toml:"port"`
		Database string `toml:"database"`
	}

	body := bytes.NewReader([]byte(`
host = "db.example.com"
port = 5432
database = "mydb"
`))

	db, err := toml.TOMLReader[Database](body)
	if err != nil {
		_, _ = fmt.Printf("Error: %v\n", err)
		return
	}

	_, _ = fmt.Printf("Database: %s@%s:%d\n", db.Database, db.Host, db.Port)
	// Output: Database: mydb@db.example.com:5432
}

// ExampleTOML_nestedTables demonstrates binding nested TOML tables.
func ExampleTOML_nestedTables() {
	type Database struct {
		Host string `toml:"host"`
		Port int    `toml:"port"`
	}

	type Server struct {
		Host string `toml:"host"`
		Port int    `toml:"port"`
	}

	type Config struct {
		Title    string   `toml:"title"`
		Server   Server   `toml:"server"`
		Database Database `toml:"database"`
	}

	body := []byte(`
title = "My Service"

[server]
host = "0.0.0.0"
port = 8080

[database]
host = "localhost"
port = 5432
`)

	config, err := toml.TOML[Config](body)
	if err != nil {
		_, _ = fmt.Printf("Error: %v\n", err)
		return
	}

	_, _ = fmt.Printf("Title: %s, Server: %s:%d, DB: %s:%d\n",
		config.Title,
		config.Server.Host, config.Server.Port,
		config.Database.Host, config.Database.Port)
	// Output: Title: My Service, Server: 0.0.0.0:8080, DB: localhost:5432
}

// ExampleTOML_arrays demonstrates binding TOML arrays.
func ExampleTOML_arrays() {
	type Config struct {
		Hosts []string `toml:"hosts"`
		Ports []int    `toml:"ports"`
	}

	body := []byte(`
hosts = ["host1.example.com", "host2.example.com"]
ports = [8080, 8081, 8082]
`)

	config, err := toml.TOML[Config](body)
	if err != nil {
		_, _ = fmt.Printf("Error: %v\n", err)
		return
	}

	_, _ = fmt.Printf("Hosts: %v, Ports: %v\n", config.Hosts, config.Ports)
	// Output: Hosts: [host1.example.com host2.example.com], Ports: [8080 8081 8082]
}

// ExampleTOML_arrayOfTables demonstrates binding TOML array of tables.
func ExampleTOML_arrayOfTables() {
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
`)

	catalog, err := toml.TOML[Catalog](body)
	if err != nil {
		_, _ = fmt.Printf("Error: %v\n", err)
		return
	}

	_, _ = fmt.Printf("Products: %d items\n", len(catalog.Products))
	_, _ = fmt.Printf("First: %s ($%d)\n", catalog.Products[0].Name, catalog.Products[0].Price)
	// Output: Products: 2 items
	// First: Widget ($100)
}

// ExampleTOML_inlineTable demonstrates binding TOML inline tables.
func ExampleTOML_inlineTable() {
	type Point struct {
		X int `toml:"x"`
		Y int `toml:"y"`
	}

	type Config struct {
		Origin Point `toml:"origin"`
	}

	body := []byte(`
origin = { x = 10, y = 20 }
`)

	config, err := toml.TOML[Config](body)
	if err != nil {
		_, _ = fmt.Printf("Error: %v\n", err)
		return
	}

	_, _ = fmt.Printf("Origin: (%d, %d)\n", config.Origin.X, config.Origin.Y)
	// Output: Origin: (10, 20)
}

// ExampleTOMLWithMetadata demonstrates accessing TOML metadata.
func ExampleTOMLWithMetadata() {
	type Config struct {
		Name string `toml:"name"`
	}

	body := []byte(`
name = "myapp"
unknown = "ignored"
`)

	config, meta, err := toml.TOMLWithMetadata[Config](body)
	if err != nil {
		_, _ = fmt.Printf("Error: %v\n", err)
		return
	}

	_, _ = fmt.Printf("Name: %s\n", config.Name)
	_, _ = fmt.Printf("Undecoded keys: %v\n", meta.Undecoded())
	// Output: Name: myapp
	// Undecoded keys: [unknown]
}

// ExampleTOML_withValidator demonstrates TOML binding with validation.
func ExampleTOML_withValidator() {
	type Config struct {
		Title string `toml:"title"`
		Port  int    `toml:"port"`
	}

	body := []byte(`
title = "My App"
port = 8080
`)

	// Create a simple validator
	validator := &simpleValidator{}

	config, err := toml.TOML[Config](body, toml.WithValidator(validator))
	if err != nil {
		_, _ = fmt.Printf("Error: %v\n", err)
		return
	}

	_, _ = fmt.Printf("Title: %s, Port: %d\n", config.Title, config.Port)
	// Output: Title: My App, Port: 8080
}

// simpleValidator is a test validator for examples.
type simpleValidator struct{}

func (v *simpleValidator) Validate(data any) error {
	return nil
}
