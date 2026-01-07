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

package proto

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"rivaas.dev/binding/proto/testdata"
)

func TestProto_BasicBinding(t *testing.T) {
	t.Parallel()

	user := &testdata.User{
		Name:   "John",
		Email:  "john@example.com",
		Age:    30,
		Active: true,
	}
	body, err := proto.Marshal(user)
	require.NoError(t, err)

	result, err := Proto[*testdata.User](body)
	require.NoError(t, err)
	assert.Equal(t, "John", result.GetName())
	assert.Equal(t, "john@example.com", result.GetEmail())
	assert.Equal(t, int32(30), result.GetAge())
	assert.True(t, result.GetActive())
}

func TestProto_NestedStructs(t *testing.T) {
	t.Parallel()

	config := &testdata.Config{
		Title: "My App",
		Server: &testdata.Server{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Database: &testdata.Database{
			Host:     "localhost",
			Port:     5432,
			Name:     "mydb",
			User:     "admin",
			Password: "secret",
		},
	}
	body, err := proto.Marshal(config)
	require.NoError(t, err)

	result, err := Proto[*testdata.Config](body)
	require.NoError(t, err)
	assert.Equal(t, "My App", result.GetTitle())
	assert.Equal(t, "0.0.0.0", result.GetServer().GetHost())
	assert.Equal(t, int32(8080), result.GetServer().GetPort())
	assert.Equal(t, "localhost", result.GetDatabase().GetHost())
	assert.Equal(t, int32(5432), result.GetDatabase().GetPort())
	assert.Equal(t, "mydb", result.GetDatabase().GetName())
}

func TestProto_RepeatedFields(t *testing.T) {
	t.Parallel()

	product := &testdata.Product{
		Name:   "Widget",
		Tags:   []string{"electronics", "gadget", "sale"},
		Prices: []int32{100, 200, 300},
	}
	body, err := proto.Marshal(product)
	require.NoError(t, err)

	result, err := Proto[*testdata.Product](body)
	require.NoError(t, err)
	assert.Equal(t, "Widget", result.GetName())
	assert.Equal(t, []string{"electronics", "gadget", "sale"}, result.GetTags())
	assert.Equal(t, []int32{100, 200, 300}, result.GetPrices())
}

func TestProto_MapFields(t *testing.T) {
	t.Parallel()

	settings := &testdata.Settings{
		Name: "MySettings",
		Metadata: map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		},
	}
	body, err := proto.Marshal(settings)
	require.NoError(t, err)

	result, err := Proto[*testdata.Settings](body)
	require.NoError(t, err)
	assert.Equal(t, "MySettings", result.GetName())
	assert.Equal(t, "value1", result.GetMetadata()["key1"])
	assert.Equal(t, "value2", result.GetMetadata()["key2"])
	assert.Equal(t, "value3", result.GetMetadata()["key3"])
}

func TestProto_InvalidData(t *testing.T) {
	t.Parallel()

	body := []byte("invalid proto data that cannot be unmarshaled")

	_, err := Proto[*testdata.User](body)
	require.Error(t, err)
}

func TestProto_EmptyBody(t *testing.T) {
	t.Parallel()

	body := []byte{}

	result, err := Proto[*testdata.User](body)
	require.NoError(t, err) // Empty body is valid proto, results in zero values
	assert.Empty(t, result.GetName())
	assert.Empty(t, result.GetEmail())
	assert.Equal(t, int32(0), result.GetAge())
	assert.False(t, result.GetActive())
}

func TestProtoTo_NonGeneric(t *testing.T) {
	t.Parallel()

	user := &testdata.User{
		Name:   "Alice",
		Email:  "alice@example.com",
		Age:    25,
		Active: false,
	}
	body, err := proto.Marshal(user)
	require.NoError(t, err)

	var result testdata.User
	err = ProtoTo(body, &result)
	require.NoError(t, err)
	assert.Equal(t, "Alice", result.GetName())
	assert.Equal(t, "alice@example.com", result.GetEmail())
	assert.Equal(t, int32(25), result.GetAge())
	assert.False(t, result.GetActive())
}

func TestProtoTo_InvalidData(t *testing.T) {
	t.Parallel()

	body := []byte("invalid proto data")

	var result testdata.User
	err := ProtoTo(body, &result)
	require.Error(t, err)
}

func TestProtoReader_FromReader(t *testing.T) {
	t.Parallel()

	user := &testdata.User{
		Name:   "Bob",
		Email:  "bob@example.com",
		Age:    35,
		Active: true,
	}
	body, err := proto.Marshal(user)
	require.NoError(t, err)

	result, err := ProtoReader[*testdata.User](bytes.NewReader(body))
	require.NoError(t, err)
	assert.Equal(t, "Bob", result.GetName())
	assert.Equal(t, "bob@example.com", result.GetEmail())
	assert.Equal(t, int32(35), result.GetAge())
	assert.True(t, result.GetActive())
}

func TestProtoReaderTo_NonGeneric(t *testing.T) {
	t.Parallel()

	config := &testdata.Config{
		Title: "Test Config",
		Server: &testdata.Server{
			Host: "192.168.1.1",
			Port: 3000,
		},
	}
	body, err := proto.Marshal(config)
	require.NoError(t, err)

	var result testdata.Config
	err = ProtoReaderTo(bytes.NewReader(body), &result)
	require.NoError(t, err)
	assert.Equal(t, "Test Config", result.GetTitle())
	assert.Equal(t, "192.168.1.1", result.GetServer().GetHost())
	assert.Equal(t, int32(3000), result.GetServer().GetPort())
}

func TestProto_WithDiscardUnknown(t *testing.T) {
	t.Parallel()

	user := &testdata.User{
		Name:   "John",
		Email:  "john@example.com",
		Age:    30,
		Active: true,
	}
	body, err := proto.Marshal(user)
	require.NoError(t, err)

	// With DiscardUnknown option
	result, err := Proto[*testdata.User](body, WithDiscardUnknown())
	require.NoError(t, err)
	assert.Equal(t, "John", result.GetName())
}

func TestProto_WithAllowPartial(t *testing.T) {
	t.Parallel()

	user := &testdata.User{
		Name: "John",
		// Other fields intentionally left empty
	}
	body, err := proto.Marshal(user)
	require.NoError(t, err)

	// With AllowPartial option
	result, err := Proto[*testdata.User](body, WithAllowPartial())
	require.NoError(t, err)
	assert.Equal(t, "John", result.GetName())
}

func TestProto_WithRecursionLimit(t *testing.T) {
	t.Parallel()

	user := &testdata.User{
		Name:   "John",
		Email:  "john@example.com",
		Age:    30,
		Active: true,
	}
	body, err := proto.Marshal(user)
	require.NoError(t, err)

	// With RecursionLimit option
	result, err := Proto[*testdata.User](body, WithRecursionLimit(1000))
	require.NoError(t, err)
	assert.Equal(t, "John", result.GetName())
}

func TestProto_MultipleOptions(t *testing.T) {
	t.Parallel()

	user := &testdata.User{
		Name:   "John",
		Email:  "john@example.com",
		Age:    30,
		Active: true,
	}
	body, err := proto.Marshal(user)
	require.NoError(t, err)

	// With multiple options
	result, err := Proto[*testdata.User](body,
		WithDiscardUnknown(),
		WithAllowPartial(),
		WithRecursionLimit(5000),
	)
	require.NoError(t, err)
	assert.Equal(t, "John", result.GetName())
}

func TestSourceGetter_Methods(t *testing.T) {
	t.Parallel()

	user := &testdata.User{
		Name:  "John",
		Email: "john@example.com",
	}
	body, err := proto.Marshal(user)
	require.NoError(t, err)

	cfg := applyOptions(nil)
	getter := &sourceGetter{body: body, cfg: cfg}

	// These methods should return empty values as proto doesn't support key-value access
	assert.Empty(t, getter.Get("name"))
	assert.Nil(t, getter.GetAll("name"))
	assert.False(t, getter.Has("name"))
}

func TestFromProto_Option(t *testing.T) {
	t.Parallel()

	user := &testdata.User{
		Name:  "John",
		Email: "john@example.com",
	}
	body, err := proto.Marshal(user)
	require.NoError(t, err)

	// FromProto should return a valid binding.Option
	opt := FromProto(body)
	assert.NotNil(t, opt)
}

func TestFromProtoReader_Option(t *testing.T) {
	t.Parallel()

	user := &testdata.User{
		Name:  "John",
		Email: "john@example.com",
	}
	body, err := proto.Marshal(user)
	require.NoError(t, err)

	// FromProtoReader should return a valid binding.Option
	opt := FromProtoReader(bytes.NewReader(body))
	assert.NotNil(t, opt)
}

func TestApplyOptions_DefaultValues(t *testing.T) {
	t.Parallel()

	cfg := applyOptions(nil)
	assert.Equal(t, 10000, cfg.recursionLimit)
	assert.False(t, cfg.allowPartial)
	assert.False(t, cfg.discardUnknown)
}

func TestConfig_ToUnmarshalOptions(t *testing.T) {
	t.Parallel()

	cfg := &config{
		allowPartial:   true,
		discardUnknown: true,
		recursionLimit: 5000,
	}

	unmarshalOpts := cfg.toUnmarshalOptions()
	assert.True(t, unmarshalOpts.AllowPartial)
	assert.True(t, unmarshalOpts.DiscardUnknown)
	assert.Equal(t, 5000, unmarshalOpts.RecursionLimit)
}
