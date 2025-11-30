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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"rivaas.dev/binding"
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
	assert.Equal(t, "John", result.Name)
	assert.Equal(t, "john@example.com", result.Email)
	assert.Equal(t, int32(30), result.Age)
	assert.True(t, result.Active)
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
	assert.Equal(t, "My App", result.Title)
	assert.Equal(t, "0.0.0.0", result.Server.Host)
	assert.Equal(t, int32(8080), result.Server.Port)
	assert.Equal(t, "localhost", result.Database.Host)
	assert.Equal(t, int32(5432), result.Database.Port)
	assert.Equal(t, "mydb", result.Database.Name)
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
	assert.Equal(t, "Widget", result.Name)
	assert.Equal(t, []string{"electronics", "gadget", "sale"}, result.Tags)
	assert.Equal(t, []int32{100, 200, 300}, result.Prices)
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
	assert.Equal(t, "MySettings", result.Name)
	assert.Equal(t, "value1", result.Metadata["key1"])
	assert.Equal(t, "value2", result.Metadata["key2"])
	assert.Equal(t, "value3", result.Metadata["key3"])
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
	assert.Equal(t, "", result.Name)
	assert.Equal(t, "", result.Email)
	assert.Equal(t, int32(0), result.Age)
	assert.False(t, result.Active)
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
	assert.Equal(t, "Alice", result.Name)
	assert.Equal(t, "alice@example.com", result.Email)
	assert.Equal(t, int32(25), result.Age)
	assert.False(t, result.Active)
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
	assert.Equal(t, "Bob", result.Name)
	assert.Equal(t, "bob@example.com", result.Email)
	assert.Equal(t, int32(35), result.Age)
	assert.True(t, result.Active)
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
	assert.Equal(t, "Test Config", result.Title)
	assert.Equal(t, "192.168.1.1", result.Server.Host)
	assert.Equal(t, int32(3000), result.Server.Port)
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
	assert.Equal(t, "John", result.Name)
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
	assert.Equal(t, "John", result.Name)
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
	assert.Equal(t, "John", result.Name)
}

func TestProto_WithValidator(t *testing.T) {
	t.Parallel()

	user := &testdata.User{
		Name:   "John",
		Email:  "john@example.com",
		Age:    30,
		Active: true,
	}
	body, err := proto.Marshal(user)
	require.NoError(t, err)

	// With passing validator
	passingValidator := &testValidator{shouldFail: false}
	result, err := Proto[*testdata.User](body, WithValidator(passingValidator))
	require.NoError(t, err)
	assert.Equal(t, "John", result.Name)

	// With failing validator
	failingValidator := &testValidator{shouldFail: true, errMsg: "validation failed"}
	_, err = Proto[*testdata.User](body, WithValidator(failingValidator))
	require.Error(t, err)

	// Verify it's a BindError
	var bindErr *binding.BindError
	require.ErrorAs(t, err, &bindErr)
	assert.Equal(t, binding.SourceProto, bindErr.Source)
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
	assert.Equal(t, "John", result.Name)
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
	assert.Equal(t, "", getter.Get("name"))
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
	assert.Nil(t, cfg.validator)
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

// testValidator is a simple validator for testing.
type testValidator struct {
	shouldFail bool
	errMsg     string
}

func (v *testValidator) Validate(data any) error {
	if v.shouldFail {
		return errors.New(v.errMsg)
	}
	return nil
}
