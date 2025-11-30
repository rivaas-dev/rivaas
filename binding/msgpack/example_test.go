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

package msgpack_test

import (
	"bytes"
	"fmt"
	"log"

	mp "github.com/vmihailenco/msgpack/v5"

	"rivaas.dev/binding/msgpack"
)

// ExampleMsgPack demonstrates basic MessagePack binding.
func ExampleMsgPack() {
	type Config struct {
		Name  string `msgpack:"name"`
		Port  int    `msgpack:"port"`
		Debug bool   `msgpack:"debug"`
	}

	// Create MessagePack data
	original := Config{
		Name:  "myapp",
		Port:  8080,
		Debug: true,
	}
	body, err := mp.Marshal(&original)
	if err != nil {
		log.Fatal(err)
	}

	config, err := msgpack.MsgPack[Config](body)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Name: %s, Port: %d, Debug: %v\n", config.Name, config.Port, config.Debug)
	// Output: Name: myapp, Port: 8080, Debug: true
}

// ExampleMsgPackTo demonstrates non-generic MessagePack binding.
func ExampleMsgPackTo() {
	type Server struct {
		Host string `msgpack:"host"`
		Port int    `msgpack:"port"`
	}

	original := Server{Host: "localhost", Port: 3000}
	body, err := mp.Marshal(&original)
	if err != nil {
		log.Fatal(err)
	}

	var server Server
	err = msgpack.MsgPackTo(body, &server)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Server: %s:%d\n", server.Host, server.Port)
	// Output: Server: localhost:3000
}

// ExampleMsgPackReader demonstrates binding from an io.Reader.
func ExampleMsgPackReader() {
	type Database struct {
		Host     string `msgpack:"host"`
		Port     int    `msgpack:"port"`
		Database string `msgpack:"database"`
	}

	original := Database{
		Host:     "db.example.com",
		Port:     5432,
		Database: "mydb",
	}
	data, err := mp.Marshal(&original)
	if err != nil {
		log.Fatal(err)
	}

	db, err := msgpack.MsgPackReader[Database](bytes.NewReader(data))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Database: %s@%s:%d\n", db.Database, db.Host, db.Port)
	// Output: Database: mydb@db.example.com:5432
}

// ExampleMsgPack_nestedStructs demonstrates binding nested MessagePack structures.
func ExampleMsgPack_nestedStructs() {
	type Address struct {
		Street string `msgpack:"street"`
		City   string `msgpack:"city"`
	}

	type Person struct {
		Name    string  `msgpack:"name"`
		Address Address `msgpack:"address"`
	}

	original := Person{
		Name: "Jane",
		Address: Address{
			Street: "123 Main St",
			City:   "Boston",
		},
	}
	body, err := mp.Marshal(&original)
	if err != nil {
		log.Fatal(err)
	}

	person, err := msgpack.MsgPack[Person](body)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Name: %s, Address: %s, %s\n",
		person.Name, person.Address.Street, person.Address.City)
	// Output: Name: Jane, Address: 123 Main St, Boston
}

// ExampleMsgPack_arrays demonstrates binding MessagePack arrays.
func ExampleMsgPack_arrays() {
	type Config struct {
		Hosts []string `msgpack:"hosts"`
		Ports []int    `msgpack:"ports"`
	}

	original := Config{
		Hosts: []string{"host1.example.com", "host2.example.com"},
		Ports: []int{8080, 8081},
	}
	body, err := mp.Marshal(&original)
	if err != nil {
		log.Fatal(err)
	}

	config, err := msgpack.MsgPack[Config](body)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Hosts: %d, Ports: %d\n", len(config.Hosts), len(config.Ports))
	// Output: Hosts: 2, Ports: 2
}

// ExampleMsgPack_withJSONTag demonstrates using JSON tags for MessagePack.
func ExampleMsgPack_withJSONTag() {
	type User struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	// Encode using JSON tags
	buf := &bytes.Buffer{}
	enc := mp.NewEncoder(buf)
	enc.SetCustomStructTag("json")
	err := enc.Encode(&User{
		Name:  "John",
		Email: "john@example.com",
	})
	if err != nil {
		log.Fatal(err)
	}

	user, err := msgpack.MsgPack[User](buf.Bytes(), msgpack.WithJSONTag())
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Name: %s, Email: %s\n", user.Name, user.Email)
	// Output: Name: John, Email: john@example.com
}

// ExampleMsgPack_maps demonstrates binding MessagePack maps.
func ExampleMsgPack_maps() {
	type Config struct {
		Settings map[string]string `msgpack:"settings"`
	}

	original := Config{
		Settings: map[string]string{
			"log_level":   "debug",
			"environment": "production",
		},
	}
	body, err := mp.Marshal(&original)
	if err != nil {
		log.Fatal(err)
	}

	config, err := msgpack.MsgPack[Config](body)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Log Level: %s\n", config.Settings["log_level"])
	// Output: Log Level: debug
}

// ExampleMsgPack_withValidator demonstrates MessagePack binding with validation.
func ExampleMsgPack_withValidator() {
	type Config struct {
		Name string `msgpack:"name"`
		Port int    `msgpack:"port"`
	}

	original := Config{Name: "myapp", Port: 8080}
	body, err := mp.Marshal(&original)
	if err != nil {
		log.Fatal(err)
	}

	// Create a simple validator
	validator := &simpleValidator{}

	config, err := msgpack.MsgPack[Config](body, msgpack.WithValidator(validator))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Name: %s, Port: %d\n", config.Name, config.Port)
	// Output: Name: myapp, Port: 8080
}

// simpleValidator is a test validator for examples.
type simpleValidator struct{}

func (v *simpleValidator) Validate(data any) error {
	return nil
}

// ExampleMsgPack_withDisallowUnknown demonstrates strict unknown field handling.
func ExampleMsgPack_withDisallowUnknown() {
	type Config struct {
		Name string `msgpack:"name"`
	}

	// Only known fields
	original := Config{Name: "myapp"}
	body, err := mp.Marshal(&original)
	if err != nil {
		log.Fatal(err)
	}

	config, err := msgpack.MsgPack[Config](body, msgpack.WithDisallowUnknown())
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Name: %s\n", config.Name)
	// Output: Name: myapp
}
