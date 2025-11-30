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

package msgpack

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v5"
)

type MsgPackUser struct {
	Name  string `msgpack:"name"`
	Email string `msgpack:"email"`
	Age   int    `msgpack:"age"`
}

type MsgPackConfig struct {
	Server  string `msgpack:"server"`
	Port    int    `msgpack:"port"`
	Enabled bool   `msgpack:"enabled"`
}

func TestMsgPack_BasicBinding(t *testing.T) {
	t.Parallel()

	user := MsgPackUser{
		Name:  "John",
		Email: "john@example.com",
		Age:   30,
	}
	body, err := msgpack.Marshal(&user)
	require.NoError(t, err)

	result, err := MsgPack[MsgPackUser](body)
	require.NoError(t, err)
	assert.Equal(t, "John", result.Name)
	assert.Equal(t, "john@example.com", result.Email)
	assert.Equal(t, 30, result.Age)
}

func TestMsgPack_GenericFunction(t *testing.T) {
	t.Parallel()

	config := MsgPackConfig{
		Server:  "localhost",
		Port:    8080,
		Enabled: true,
	}
	body, err := msgpack.Marshal(&config)
	require.NoError(t, err)

	result, err := MsgPack[MsgPackConfig](body)
	require.NoError(t, err)
	assert.Equal(t, "localhost", result.Server)
	assert.Equal(t, 8080, result.Port)
	assert.True(t, result.Enabled)
}

func TestMsgPackTo_NonGeneric(t *testing.T) {
	user := MsgPackUser{
		Name:  "Alice",
		Email: "alice@example.com",
		Age:   25,
	}
	body, err := msgpack.Marshal(&user)
	require.NoError(t, err)

	var result MsgPackUser
	err = MsgPackTo(body, &result)
	require.NoError(t, err)
	assert.Equal(t, "Alice", result.Name)
	assert.Equal(t, "alice@example.com", result.Email)
	assert.Equal(t, 25, result.Age)
}

func TestMsgPackReader_FromReader(t *testing.T) {
	t.Parallel()

	user := MsgPackUser{
		Name:  "Bob",
		Email: "bob@example.com",
		Age:   35,
	}
	body, err := msgpack.Marshal(&user)
	require.NoError(t, err)

	result, err := MsgPackReader[MsgPackUser](bytes.NewReader(body))
	require.NoError(t, err)
	assert.Equal(t, "Bob", result.Name)
	assert.Equal(t, "bob@example.com", result.Email)
	assert.Equal(t, 35, result.Age)
}

func TestMsgPackReaderTo_NonGeneric(t *testing.T) {
	t.Parallel()

	config := MsgPackConfig{
		Server:  "192.168.1.1",
		Port:    3000,
		Enabled: false,
	}
	body, err := msgpack.Marshal(&config)
	require.NoError(t, err)

	var result MsgPackConfig
	err = MsgPackReaderTo(bytes.NewReader(body), &result)
	require.NoError(t, err)
	assert.Equal(t, "192.168.1.1", result.Server)
	assert.Equal(t, 3000, result.Port)
	assert.False(t, result.Enabled)
}

func TestMsgPack_InvalidData(t *testing.T) {
	t.Parallel()

	body := []byte("invalid msgpack data")

	_, err := MsgPack[MsgPackUser](body)
	require.Error(t, err)
}

func TestMsgPack_WithJSONTag(t *testing.T) {
	type JSONTaggedUser struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Age   int    `json:"age"`
	}

	// Encode using JSON tags
	buf := &bytes.Buffer{}
	enc := msgpack.NewEncoder(buf)
	enc.SetCustomStructTag("json")
	err := enc.Encode(&JSONTaggedUser{
		Name:  "Jane",
		Email: "jane@example.com",
		Age:   28,
	})
	require.NoError(t, err)

	result, err := MsgPack[JSONTaggedUser](buf.Bytes(), WithJSONTag())
	require.NoError(t, err)
	assert.Equal(t, "Jane", result.Name)
	assert.Equal(t, "jane@example.com", result.Email)
	assert.Equal(t, 28, result.Age)
}

func TestMsgPack_NestedStructs(t *testing.T) {
	t.Parallel()

	type Address struct {
		Street string `msgpack:"street"`
		City   string `msgpack:"city"`
	}
	type Person struct {
		Name    string  `msgpack:"name"`
		Address Address `msgpack:"address"`
	}

	person := Person{
		Name: "Jane",
		Address: Address{
			Street: "123 Main St",
			City:   "Boston",
		},
	}
	body, err := msgpack.Marshal(&person)
	require.NoError(t, err)

	result, err := MsgPack[Person](body)
	require.NoError(t, err)
	assert.Equal(t, "Jane", result.Name)
	assert.Equal(t, "123 Main St", result.Address.Street)
	assert.Equal(t, "Boston", result.Address.City)
}

func TestMsgPack_ArrayBinding(t *testing.T) {
	t.Parallel()

	type Config struct {
		Hosts []string `msgpack:"hosts"`
		Ports []int    `msgpack:"ports"`
	}

	config := Config{
		Hosts: []string{"host1.example.com", "host2.example.com", "host3.example.com"},
		Ports: []int{8080, 8081, 8082},
	}
	body, err := msgpack.Marshal(&config)
	require.NoError(t, err)

	result, err := MsgPack[Config](body)
	require.NoError(t, err)
	assert.Equal(t, []string{"host1.example.com", "host2.example.com", "host3.example.com"}, result.Hosts)
	assert.Equal(t, []int{8080, 8081, 8082}, result.Ports)
}

func TestMsgPack_MapBinding(t *testing.T) {
	t.Parallel()

	type Config struct {
		Settings map[string]string `msgpack:"settings"`
	}

	config := Config{
		Settings: map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		},
	}
	body, err := msgpack.Marshal(&config)
	require.NoError(t, err)

	result, err := MsgPack[Config](body)
	require.NoError(t, err)
	assert.Equal(t, "value1", result.Settings["key1"])
	assert.Equal(t, "value2", result.Settings["key2"])
	assert.Equal(t, "value3", result.Settings["key3"])
}

func TestMsgPack_WithDisallowUnknown(t *testing.T) {
	t.Parallel()

	// Create msgpack with an extra field
	type Source struct {
		Name    string `msgpack:"name"`
		Email   string `msgpack:"email"`
		Unknown string `msgpack:"unknown_field"`
	}

	type Target struct {
		Name  string `msgpack:"name"`
		Email string `msgpack:"email"`
	}

	src := Source{
		Name:    "John",
		Email:   "john@example.com",
		Unknown: "extra",
	}
	body, err := msgpack.Marshal(&src)
	require.NoError(t, err)

	// Without DisallowUnknown, this should work
	result, err := MsgPack[Target](body)
	require.NoError(t, err)
	assert.Equal(t, "John", result.Name)

	// With DisallowUnknown, this should fail
	_, err = MsgPack[Target](body, WithDisallowUnknown())
	require.Error(t, err)
}
