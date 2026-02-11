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

package toml

import (
	"bytes"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/binding"
)

type TOMLUser struct {
	Name  string `toml:"name"`
	Email string `toml:"email"`
	Age   int    `toml:"age"`
}

type TOMLConfig struct {
	Title   string `toml:"title"`
	Server  string `toml:"server"`
	Port    int    `toml:"port"`
	Enabled bool   `toml:"enabled"`
}

func TestTOML_BasicBinding(t *testing.T) {
	t.Parallel()

	body := []byte(`
name = "John"
email = "john@example.com"
age = 30
`)

	user, err := TOML[TOMLUser](body)
	require.NoError(t, err)
	assert.Equal(t, "John", user.Name)
	assert.Equal(t, "john@example.com", user.Email)
	assert.Equal(t, 30, user.Age)
}

func TestTOML_GenericFunction(t *testing.T) {
	t.Parallel()

	body := []byte(`
title = "My Config"
server = "localhost"
port = 8080
enabled = true
`)

	config, err := TOML[TOMLConfig](body)
	require.NoError(t, err)
	assert.Equal(t, "My Config", config.Title)
	assert.Equal(t, "localhost", config.Server)
	assert.Equal(t, 8080, config.Port)
	assert.True(t, config.Enabled)
}

func TestTOMLTo_NonGeneric(t *testing.T) {
	t.Parallel()

	body := []byte(`
name = "Alice"
email = "alice@example.com"
age = 25
`)

	var user TOMLUser
	err := TOMLTo(body, &user)
	require.NoError(t, err)
	assert.Equal(t, "Alice", user.Name)
	assert.Equal(t, "alice@example.com", user.Email)
	assert.Equal(t, 25, user.Age)
}

func TestTOMLReader_FromReader(t *testing.T) {
	t.Parallel()

	body := bytes.NewReader([]byte(`
name = "Bob"
email = "bob@example.com"
age = 35
`))

	user, err := TOMLReader[TOMLUser](body)
	require.NoError(t, err)
	assert.Equal(t, "Bob", user.Name)
	assert.Equal(t, "bob@example.com", user.Email)
	assert.Equal(t, 35, user.Age)
}

func TestTOMLReaderTo_NonGeneric(t *testing.T) {
	t.Parallel()

	body := bytes.NewReader([]byte(`
title = "Test Config"
server = "192.168.1.1"
port = 3000
enabled = false
`))

	var config TOMLConfig
	err := TOMLReaderTo(body, &config)
	require.NoError(t, err)
	assert.Equal(t, "Test Config", config.Title)
	assert.Equal(t, "192.168.1.1", config.Server)
	assert.Equal(t, 3000, config.Port)
	assert.False(t, config.Enabled)
}

func TestTOML_InvalidTOML(t *testing.T) {
	t.Parallel()

	body := []byte(`
name = John  # missing quotes
`)

	_, err := TOML[TOMLUser](body)
	require.Error(t, err)
}

func TestTOMLWithMetadata_DecodesKeys(t *testing.T) {
	t.Parallel()

	body := []byte(`
name = "John"
email = "john@example.com"
age = 30
unknown_field = "ignored"
`)

	user, meta, err := TOMLWithMetadata[TOMLUser](body)
	require.NoError(t, err)
	assert.Equal(t, "John", user.Name)
	assert.Equal(t, "john@example.com", user.Email)
	assert.Equal(t, 30, user.Age)

	// Check for undecoded keys
	undecoded := meta.Undecoded()
	assert.Len(t, undecoded, 1)
	assert.Equal(t, "unknown_field", undecoded[0].String())
}

func TestTOML_NestedStructs(t *testing.T) {
	t.Parallel()

	type Database struct {
		Host string `toml:"host"`
		Port int    `toml:"port"`
	}
	type Config struct {
		Title    string   `toml:"title"`
		Database Database `toml:"database"`
	}

	body := []byte(`
title = "My App"

[database]
host = "localhost"
port = 5432
`)

	config, err := TOML[Config](body)
	require.NoError(t, err)
	assert.Equal(t, "My App", config.Title)
	assert.Equal(t, "localhost", config.Database.Host)
	assert.Equal(t, 5432, config.Database.Port)
}

func TestTOML_ArrayBinding(t *testing.T) {
	t.Parallel()

	type Config struct {
		Hosts []string `toml:"hosts"`
		Ports []int    `toml:"ports"`
	}

	body := []byte(`
hosts = ["host1.example.com", "host2.example.com", "host3.example.com"]
ports = [8080, 8081, 8082]
`)

	config, err := TOML[Config](body)
	require.NoError(t, err)
	assert.Equal(t, []string{"host1.example.com", "host2.example.com", "host3.example.com"}, config.Hosts)
	assert.Equal(t, []int{8080, 8081, 8082}, config.Ports)
}

func TestTOML_InlineTable(t *testing.T) {
	t.Parallel()

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

	config, err := TOML[Config](body)
	require.NoError(t, err)
	assert.Equal(t, 10, config.Origin.X)
	assert.Equal(t, 20, config.Origin.Y)
}

func TestTOML_ArrayOfTables(t *testing.T) {
	t.Parallel()

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
`)

	catalog, err := TOML[Catalog](body)
	require.NoError(t, err)
	require.Len(t, catalog.Products, 3)
	assert.Equal(t, "Widget", catalog.Products[0].Name)
	assert.Equal(t, 100, catalog.Products[0].Price)
	assert.Equal(t, "Gadget", catalog.Products[1].Name)
	assert.Equal(t, 200, catalog.Products[1].Price)
	assert.Equal(t, "Gizmo", catalog.Products[2].Name)
	assert.Equal(t, 300, catalog.Products[2].Price)
}

// TestFromTOML tests binding.BindTo with toml.FromTOML as a source.
func TestFromTOML(t *testing.T) {
	t.Parallel()

	type Request struct {
		Page  int    `query:"page"`
		Title string `toml:"title"`
		Name  string `toml:"name"`
	}
	body := []byte(`
title = "My App"
name = "myapp"
`)
	values := url.Values{}
	values.Set("page", "3")
	var out Request
	err := binding.BindTo(&out,
		binding.FromQuery(values),
		FromTOML(body),
	)
	require.NoError(t, err)
	assert.Equal(t, 3, out.Page)
}

// TestFromTOMLReader tests binding.BindTo with toml.FromTOMLReader as a source.
func TestFromTOMLReader(t *testing.T) {
	t.Parallel()

	type Request struct {
		Page int    `query:"page"`
		Name string `toml:"name"`
	}
	body := []byte(`name = "reader-app"`)
	values := url.Values{}
	values.Set("page", "1")
	var out Request
	err := binding.BindTo(&out,
		binding.FromQuery(values),
		FromTOMLReader(bytes.NewReader(body)),
	)
	require.NoError(t, err)
	assert.Equal(t, 1, out.Page)
}
