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

package binding

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type XMLUser struct {
	Name  string `xml:"name"`
	Email string `xml:"email"`
	Age   int    `xml:"age"`
}

type XMLConfig struct {
	Server  string `xml:"server"`
	Port    int    `xml:"port"`
	Enabled bool   `xml:"enabled"`
}

func TestXML_BasicBinding(t *testing.T) {
	t.Parallel()
	body := []byte(`<XMLUser><name>John</name><email>john@example.com</email><age>30</age></XMLUser>`)

	user, err := XML[XMLUser](body)
	require.NoError(t, err)
	assert.Equal(t, "John", user.Name)
	assert.Equal(t, "john@example.com", user.Email)
	assert.Equal(t, 30, user.Age)
}

func TestXML_GenericFunction(t *testing.T) {
	t.Parallel()
	body := []byte(`<XMLConfig><server>localhost</server><port>8080</port><enabled>true</enabled></XMLConfig>`)

	config, err := XML[XMLConfig](body)
	require.NoError(t, err)
	assert.Equal(t, "localhost", config.Server)
	assert.Equal(t, 8080, config.Port)
	assert.True(t, config.Enabled)
}

func TestXMLTo_NonGeneric(t *testing.T) {
	t.Parallel()
	body := []byte(`<XMLUser><name>Alice</name><email>alice@example.com</email><age>25</age></XMLUser>`)

	var user XMLUser
	err := XMLTo(body, &user)
	require.NoError(t, err)
	assert.Equal(t, "Alice", user.Name)
	assert.Equal(t, "alice@example.com", user.Email)
	assert.Equal(t, 25, user.Age)
}

func TestXMLReader_FromReader(t *testing.T) {
	t.Parallel()
	body := bytes.NewReader([]byte(`<XMLUser><name>Bob</name><email>bob@example.com</email><age>35</age></XMLUser>`))

	user, err := XMLReader[XMLUser](body)
	require.NoError(t, err)
	assert.Equal(t, "Bob", user.Name)
	assert.Equal(t, "bob@example.com", user.Email)
	assert.Equal(t, 35, user.Age)
}

func TestXMLReaderTo_NonGeneric(t *testing.T) {
	t.Parallel()
	body := bytes.NewReader([]byte(`<XMLConfig><server>192.168.1.1</server><port>3000</port><enabled>false</enabled></XMLConfig>`))

	var config XMLConfig
	err := XMLReaderTo(body, &config)
	require.NoError(t, err)
	assert.Equal(t, "192.168.1.1", config.Server)
	assert.Equal(t, 3000, config.Port)
	assert.False(t, config.Enabled)
}

func TestXML_InvalidXML(t *testing.T) {
	t.Parallel()
	body := []byte(`<XMLUser><name>John</name>`)

	_, err := XML[XMLUser](body)
	require.Error(t, err)
}

func TestXML_WithStrict(t *testing.T) {
	t.Parallel()
	// Test with strict mode enabled
	body := []byte(`<XMLUser><name>John</name><email>john@example.com</email><age>30</age></XMLUser>`)

	user, err := XML[XMLUser](body, WithXMLStrict())
	require.NoError(t, err)
	assert.Equal(t, "John", user.Name)
}

func TestXML_NestedStructs(t *testing.T) {
	t.Parallel()
	type Address struct {
		Street string `xml:"street"`
		City   string `xml:"city"`
	}
	type Person struct {
		Name    string  `xml:"name"`
		Address Address `xml:"address"`
	}

	body := []byte(`<Person><name>Jane</name><address><street>123 Main St</street><city>Boston</city></address></Person>`)

	person, err := XML[Person](body)
	require.NoError(t, err)
	assert.Equal(t, "Jane", person.Name)
	assert.Equal(t, "123 Main St", person.Address.Street)
	assert.Equal(t, "Boston", person.Address.City)
}

func TestFromXML_MultiSourceBinding(t *testing.T) {
	t.Parallel()
	type Request struct {
		QueryParam string `query:"q"`
		Name       string `xml:"name"`
		Email      string `xml:"email"`
	}

	xmlBody := []byte(`<Request><name>John</name><email>john@example.com</email></Request>`)

	req, err := Bind[Request](
		FromQuery(map[string][]string{"q": {"search"}}),
		FromXML(xmlBody),
	)
	require.NoError(t, err)
	assert.Equal(t, "search", req.QueryParam)
	assert.Equal(t, "John", req.Name)
	assert.Equal(t, "john@example.com", req.Email)
}

func TestXMLWith_Binder(t *testing.T) {
	t.Parallel()
	binder := MustNew()

	body := []byte(`<XMLUser><name>Test</name><email>test@example.com</email><age>20</age></XMLUser>`)

	user, err := XMLWith[XMLUser](binder, body)
	require.NoError(t, err)
	assert.Equal(t, "Test", user.Name)
	assert.Equal(t, "test@example.com", user.Email)
	assert.Equal(t, 20, user.Age)
}

func TestBinder_XMLTo(t *testing.T) {
	t.Parallel()
	binder := MustNew()

	body := []byte(`<XMLUser><name>Test</name><email>test@example.com</email><age>20</age></XMLUser>`)

	var user XMLUser
	err := binder.XMLTo(body, &user)
	require.NoError(t, err)
	assert.Equal(t, "Test", user.Name)
}

func TestBinder_XMLReaderTo(t *testing.T) {
	t.Parallel()
	binder := MustNew()

	body := bytes.NewReader([]byte(`<XMLUser><name>Test</name><email>test@example.com</email><age>20</age></XMLUser>`))

	var user XMLUser
	err := binder.XMLReaderTo(body, &user)
	require.NoError(t, err)
	assert.Equal(t, "Test", user.Name)
}
