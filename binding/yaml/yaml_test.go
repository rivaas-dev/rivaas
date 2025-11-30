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

package yaml

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type YAMLUser struct {
	Name  string `yaml:"name"`
	Email string `yaml:"email"`
	Age   int    `yaml:"age"`
}

type YAMLConfig struct {
	Server  string `yaml:"server"`
	Port    int    `yaml:"port"`
	Enabled bool   `yaml:"enabled"`
}

func TestYAML_BasicBinding(t *testing.T) {
	t.Parallel()

	body := []byte(`
name: John
email: john@example.com
age: 30
`)

	user, err := YAML[YAMLUser](body)
	require.NoError(t, err)
	assert.Equal(t, "John", user.Name)
	assert.Equal(t, "john@example.com", user.Email)
	assert.Equal(t, 30, user.Age)
}

func TestYAML_GenericFunction(t *testing.T) {
	t.Parallel()

	body := []byte(`
server: localhost
port: 8080
enabled: true
`)

	config, err := YAML[YAMLConfig](body)
	require.NoError(t, err)
	assert.Equal(t, "localhost", config.Server)
	assert.Equal(t, 8080, config.Port)
	assert.True(t, config.Enabled)
}

func TestYAMLTo_NonGeneric(t *testing.T) {
	t.Parallel()

	body := []byte(`
name: Alice
email: alice@example.com
age: 25
`)

	var user YAMLUser
	err := YAMLTo(body, &user)
	require.NoError(t, err)
	assert.Equal(t, "Alice", user.Name)
	assert.Equal(t, "alice@example.com", user.Email)
	assert.Equal(t, 25, user.Age)
}

func TestYAMLReader_FromReader(t *testing.T) {
	t.Parallel()

	body := bytes.NewReader([]byte(`
name: Bob
email: bob@example.com
age: 35
`))

	user, err := YAMLReader[YAMLUser](body)
	require.NoError(t, err)
	assert.Equal(t, "Bob", user.Name)
	assert.Equal(t, "bob@example.com", user.Email)
	assert.Equal(t, 35, user.Age)
}

func TestYAMLReaderTo_NonGeneric(t *testing.T) {
	t.Parallel()

	body := bytes.NewReader([]byte(`
server: 192.168.1.1
port: 3000
enabled: false
`))

	var config YAMLConfig
	err := YAMLReaderTo(body, &config)
	require.NoError(t, err)
	assert.Equal(t, "192.168.1.1", config.Server)
	assert.Equal(t, 3000, config.Port)
	assert.False(t, config.Enabled)
}

func TestYAML_InvalidYAML(t *testing.T) {
	t.Parallel()

	// Completely malformed YAML
	body := []byte(`
name: [invalid: unclosed
`)

	_, err := YAML[YAMLUser](body)
	require.Error(t, err)
}

func TestYAML_WithStrict(t *testing.T) {
	t.Parallel()

	// Test with strict mode enabled - without unknown fields it should work
	body := []byte(`
name: John
email: john@example.com
age: 30
`)

	user, err := YAML[YAMLUser](body, WithStrict())
	require.NoError(t, err)
	assert.Equal(t, "John", user.Name)
}

func TestYAML_WithStrict_UnknownField(t *testing.T) {
	t.Parallel()

	body := []byte(`
name: John
email: john@example.com
age: 30
unknown_field: should_error
`)

	_, err := YAMLReader[YAMLUser](bytes.NewReader(body), WithStrict())
	require.Error(t, err)
}

func TestYAML_NestedStructs(t *testing.T) {
	t.Parallel()

	type Address struct {
		Street string `yaml:"street"`
		City   string `yaml:"city"`
	}
	type Person struct {
		Name    string  `yaml:"name"`
		Address Address `yaml:"address"`
	}

	body := []byte(`
name: Jane
address:
  street: 123 Main St
  city: Boston
`)

	person, err := YAML[Person](body)
	require.NoError(t, err)
	assert.Equal(t, "Jane", person.Name)
	assert.Equal(t, "123 Main St", person.Address.Street)
	assert.Equal(t, "Boston", person.Address.City)
}

func TestYAML_ArrayBinding(t *testing.T) {
	t.Parallel()

	type Config struct {
		Hosts []string `yaml:"hosts"`
		Ports []int    `yaml:"ports"`
	}

	body := []byte(`
hosts:
  - host1.example.com
  - host2.example.com
  - host3.example.com
ports:
  - 8080
  - 8081
  - 8082
`)

	config, err := YAML[Config](body)
	require.NoError(t, err)
	assert.Equal(t, []string{"host1.example.com", "host2.example.com", "host3.example.com"}, config.Hosts)
	assert.Equal(t, []int{8080, 8081, 8082}, config.Ports)
}

func TestYAML_MapBinding(t *testing.T) {
	t.Parallel()

	type Config struct {
		Settings map[string]string `yaml:"settings"`
	}

	body := []byte(`
settings:
  key1: value1
  key2: value2
  key3: value3
`)

	config, err := YAML[Config](body)
	require.NoError(t, err)
	assert.Equal(t, "value1", config.Settings["key1"])
	assert.Equal(t, "value2", config.Settings["key2"])
	assert.Equal(t, "value3", config.Settings["key3"])
}
