// Copyright 2025 The Rivaas Authors
// Copyright 2025 Company.info B.V.
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

package source

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/log"
	"github.com/testcontainers/testcontainers-go/modules/consul"

	"rivaas.dev/config/codec"
)

// ConsulSourceTestSuite is a test suite for the Consul source
type ConsulSourceTestSuite struct {
	suite.Suite
	consul *consul.ConsulContainer
	client *api.Client
}

// SetupSuite sets up the test suite
func (s *ConsulSourceTestSuite) SetupSuite() {
	ctx := context.Background()

	// Start Consul container with random port
	container, err := consul.Run(ctx, "hashicorp/consul:1.15", testcontainers.WithLogger(log.TestLogger(s.T())))
	s.Require().NoError(err)
	s.consul = container

	endpoint, err := container.ApiEndpoint(ctx)
	s.Require().NoError(err)

	// Set the Consul HTTP address environment variable to allow Config to connect to the Consul server
	s.T().Setenv("CONSUL_HTTP_ADDR", endpoint)

	// Create Consul client
	config := api.DefaultConfig()
	config.Address = endpoint
	s.client, err = api.NewClient(config)
	s.Require().NoError(err)
}

// TearDownSuite tears down the test suite
func (s *ConsulSourceTestSuite) TearDownSuite() {
	ctx := context.Background()
	if s.consul != nil {
		s.Require().NoError(s.consul.Terminate(ctx))
	}
}

// TestConsulSourceTestSuite runs the test suite
func TestConsulSourceTestSuite(t *testing.T) {
	suite.Run(t, new(ConsulSourceTestSuite))
}

// TestLoad_ValuePresent tests the Load method with a value present
func (s *ConsulSourceTestSuite) TestLoad_ValuePresent() {
	// Set up test data
	key := "test/value-present"
	value := `{"foo": "bar"}`
	_, err := s.client.KV().Put(&api.KVPair{
		Key:   key,
		Value: []byte(value),
	}, nil)
	s.Require().NoError(err)

	// Create Consul source
	consul, err := NewConsul(key, &codec.JSONCodec{}, nil)
	s.Require().NoError(err)

	// Load configuration
	conf, err := consul.Load(context.Background())
	s.Require().NoError(err)
	s.Equal("bar", conf["foo"])
}

// TestLoad_ValueAbsent tests the Load method with a value absent
func (s *ConsulSourceTestSuite) TestLoad_ValueAbsent() {
	// Create Consul source with non-existent key
	consul, err := NewConsul("test/value-absent", &codec.JSONCodec{}, nil)
	s.Require().NoError(err)

	// Load configuration
	conf, err := consul.Load(context.Background())
	s.Require().NoError(err)
	s.Empty(conf)
}

// TestLoad_DecodeError tests the Load method with a decode error
func (s *ConsulSourceTestSuite) TestLoad_DecodeError() {
	// Set up test data with invalid JSON
	key := "test/decode-error"
	value := `{"foo": "bar"` // Invalid JSON
	_, err := s.client.KV().Put(&api.KVPair{
		Key:   key,
		Value: []byte(value),
	}, nil)
	s.Require().NoError(err)

	// Create Consul source
	consul, err := NewConsul(key, &codec.JSONCodec{}, nil)
	s.Require().NoError(err)

	// Load configuration
	_, err = consul.Load(context.Background())
	s.Require().Error(err)
	s.Contains(err.Error(), "failed to decode consul value: unexpected end of JSON input")
}

// TestLoad_WithJSONValue tests the Load method with a JSON value
func (s *ConsulSourceTestSuite) TestLoad_WithJSONValue() {
	// Set up test data
	key := "test/config"
	value := `{"foo": "bar", "baz": 42}`
	_, err := s.client.KV().Put(&api.KVPair{
		Key:   key,
		Value: []byte(value),
	}, nil)
	s.Require().NoError(err)

	// Create Consul source
	consul, err := NewConsul(key, &codec.JSONCodec{}, nil)
	s.Require().NoError(err)

	// Load configuration
	conf, err := consul.Load(context.Background())
	s.Require().NoError(err)
	s.Equal("bar", conf["foo"])
	s.Equal(float64(42), conf["baz"])
}

// TestLoad_WithYAMLValue tests the Load method with a YAML value
func (s *ConsulSourceTestSuite) TestLoad_WithYAMLValue() {
	// Set up test data
	key := "test/yaml"
	value := `
foo: bar
baz: 42
nested:
  key: value
`
	_, err := s.client.KV().Put(&api.KVPair{
		Key:   key,
		Value: []byte(value),
	}, nil)
	s.Require().NoError(err)

	// Create Consul source
	consul, err := NewConsul(key, &codec.YAMLCodec{}, nil)
	s.Require().NoError(err)

	// Load configuration
	conf, err := consul.Load(context.Background())
	s.Require().NoError(err)
	s.Equal("bar", conf["foo"])
	s.Equal(uint64(42), conf["baz"])
	nested, ok := conf["nested"].(map[string]any)
	s.Require().True(ok)
	s.Equal("value", nested["key"])
}

// TestLoad_WithCasterCodec tests the Load method with a caster codec
func (s *ConsulSourceTestSuite) TestLoad_WithCasterCodec() {
	// Set up test data
	key := "test/caster"
	value := `42`
	_, err := s.client.KV().Put(&api.KVPair{
		Key:   key,
		Value: []byte(value),
	}, nil)
	s.Require().NoError(err)

	// Create Consul source with caster codec
	consul, err := NewConsul(key, codec.NewCaster(codec.CastTypeFloat64), nil)
	s.Require().NoError(err)

	// Load configuration
	conf, err := consul.Load(context.Background())
	s.Require().NoError(err)
	s.Equal(float64(42), conf["caster"])
}

// TestLoad_WithContextTimeout tests the Load method with a context timeout
func (s *ConsulSourceTestSuite) TestLoad_WithContextTimeout() {
	// Create mock KV that delays for 100ms
	mockKV := &mockConsulKV{delay: 100 * time.Millisecond}

	// Create Consul source with mock KV
	consul, err := NewConsul("test/timeout", &codec.JSONCodec{}, mockKV)
	s.Require().NoError(err)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Load configuration with timeout
	_, err = consul.Load(ctx)
	s.Require().Error(err)
	s.Contains(err.Error(), "context deadline exceeded")
}

// TestLoad_WithNonExistentKey tests the Load method with a non-existent key
func (s *ConsulSourceTestSuite) TestLoad_WithNonExistentKey() {
	// Create Consul source with non-existent key
	consul, err := NewConsul("non/existent/key", &codec.JSONCodec{}, nil)
	s.Require().NoError(err)

	// Load configuration
	conf, err := consul.Load(context.Background())
	s.Require().NoError(err)
	s.Empty(conf)
}

// TestLoad_WithClientInitFailure tests the Load method with a client initialization failure
func (s *ConsulSourceTestSuite) TestLoad_WithClientInitFailure() {
	// Create a mock KV that will be used to simulate client initialization failure
	mockKV := &mockConsulKV{err: fmt.Errorf("client initialization failed")}

	// Create Consul source with mock KV
	consul, err := NewConsul("test/key", &codec.JSONCodec{}, mockKV)
	s.Require().NoError(err)

	// Load configuration
	_, err = consul.Load(context.Background())
	s.Require().Error(err)
	s.Contains(err.Error(), "client initialization failed")
}

// TestLoad_WithKVOperationFailure tests the Load method with a KV operation failure
func (s *ConsulSourceTestSuite) TestLoad_WithKVOperationFailure() {
	// Create mock KV that fails
	mockKV := &mockConsulKV{err: errors.New("KV operation failed")}

	// Create Consul source with mock KV
	consul, err := NewConsul("test/key", &codec.JSONCodec{}, mockKV)
	s.Require().NoError(err)

	// Load configuration
	_, err = consul.Load(context.Background())
	s.Require().Error(err)
	s.Contains(err.Error(), "KV operation failed")
}

// TestLoad_WithSpecialCharacters tests the Load method with special characters in the key
func (s *ConsulSourceTestSuite) TestLoad_WithSpecialCharacters() {
	// Set up test data with special characters in key
	key := "test/special/chars/!@#$%^&*()"
	value := `{"foo": "bar"}`
	_, err := s.client.KV().Put(&api.KVPair{
		Key:   key,
		Value: []byte(value),
	}, nil)
	s.Require().NoError(err)

	// Create Consul source
	consul, err := NewConsul(key, &codec.JSONCodec{}, nil)
	s.Require().NoError(err)

	// Load configuration
	conf, err := consul.Load(context.Background())
	s.Require().NoError(err)
	s.Equal("bar", conf["foo"])
}

// TestLoad_WithEmptyValue tests the Load method with an empty value
func (s *ConsulSourceTestSuite) TestLoad_WithEmptyValue() {
	// Set up test data with empty value
	key := "test/empty-value"
	_, err := s.client.KV().Put(&api.KVPair{
		Key:   key,
		Value: []byte{},
	}, nil)
	s.Require().NoError(err)

	// Create Consul source
	consul, err := NewConsul(key, &codec.JSONCodec{}, nil)
	s.Require().NoError(err)

	// Load configuration
	_, err = consul.Load(context.Background())
	s.Require().Error(err)
	s.Contains(err.Error(), "failed to decode consul value")
	s.Contains(err.Error(), "unexpected end of JSON input")
}

func (s *ConsulSourceTestSuite) TestLoad_WithLargeValue() {
	// Set up test data with large value (500KB, just under Consul's 512KB limit)
	key := "test/large-value"
	largeValue := make([]byte, 0, 500*1024) // 500KB
	for i := range largeValue {
		largeValue[i] = byte(i % 256)
	}
	_, err := s.client.KV().Put(&api.KVPair{
		Key:   key,
		Value: largeValue,
	}, nil)
	s.Require().NoError(err)

	// Create Consul source
	consul, err := NewConsul(key, &codec.JSONCodec{}, nil)
	s.Require().NoError(err)

	// Load configuration
	_, err = consul.Load(context.Background())
	s.Require().Error(err) // Should fail due to invalid JSON
	s.Contains(err.Error(), "failed to decode consul value")
}

// TestLoad_WithBinaryValue tests the Load method with a binary value
func (s *ConsulSourceTestSuite) TestLoad_WithBinaryValue() {
	// Set up test data with binary value
	key := "test/binary-value"
	binaryValue := []byte{0x00, 0x01, 0x02, 0x03, 0x04}
	_, err := s.client.KV().Put(&api.KVPair{
		Key:   key,
		Value: binaryValue,
	}, nil)
	s.Require().NoError(err)

	// Create Consul source with caster codec
	consul, err := NewConsul(key, codec.NewCaster(codec.CastTypeString), nil)
	s.Require().NoError(err)

	// Load configuration
	conf, err := consul.Load(context.Background())
	s.Require().NoError(err)
	s.Equal(string(binaryValue), conf["binary-value"])
}

// TestLoad_WithMultipleValues tests the Load method with multiple values
func (s *ConsulSourceTestSuite) TestLoad_WithMultipleValues() {
	// Set up test data with multiple values
	keys := []string{
		"test/multi/1",
		"test/multi/2",
		"test/multi/3",
	}
	values := []string{
		`{"foo": "bar1"}`,
		`{"foo": "bar2"}`,
		`{"foo": "bar3"}`,
	}

	for i, key := range keys {
		_, err := s.client.KV().Put(&api.KVPair{
			Key:   key,
			Value: []byte(values[i]),
		}, nil)
		s.Require().NoError(err)
	}

	// Test each key
	for i, key := range keys {
		consul, err := NewConsul(key, &codec.JSONCodec{}, nil)
		s.Require().NoError(err)

		conf, err := consul.Load(context.Background())
		s.Require().NoError(err)
		s.Equal(fmt.Sprintf("bar%d", i+1), conf["foo"])
	}
}

// TestLoad_WithNestedValues tests the Load method with nested values
func (s *ConsulSourceTestSuite) TestLoad_WithNestedValues() {
	// Set up test data with nested values
	key := "test/nested"
	value := `{
		"level1": {
			"level2": {
				"level3": {
					"value": "deep"
				}
			}
		}
	}`
	_, err := s.client.KV().Put(&api.KVPair{
		Key:   key,
		Value: []byte(value),
	}, nil)
	s.Require().NoError(err)

	// Create Consul source
	consul, err := NewConsul(key, &codec.JSONCodec{}, nil)
	s.Require().NoError(err)

	// Load configuration
	conf, err := consul.Load(context.Background())
	s.Require().NoError(err)

	level1, ok := conf["level1"].(map[string]any)
	s.True(ok)
	level2, ok := level1["level2"].(map[string]any)
	s.True(ok)
	level3, ok := level2["level3"].(map[string]any)
	s.True(ok)
	s.Equal("deep", level3["value"])
}

// TestLoad_WithConsulError tests the Load method with a Consul error
func (s *ConsulSourceTestSuite) TestLoad_WithConsulError() {
	// Create mock KV that returns a Consul error
	mockKV := &mockConsulKV{err: fmt.Errorf("consul error: connection refused")}

	// Create Consul source with mock KV
	consul, err := NewConsul("test/key", &codec.JSONCodec{}, mockKV)
	s.Require().NoError(err)

	// Load configuration
	_, err = consul.Load(context.Background())
	s.Require().Error(err)
	s.Contains(err.Error(), "consul error: connection refused")
}

// TestLoad_WithNilValue tests the Load method with a nil value
func (s *ConsulSourceTestSuite) TestLoad_WithNilValue() {
	// Set up test data with nil value
	key := "test/nil-value"
	_, err := s.client.KV().Put(&api.KVPair{
		Key:   key,
		Value: nil,
	}, nil)
	s.Require().NoError(err)

	// Create Consul source
	consul, err := NewConsul(key, &codec.JSONCodec{}, nil)
	s.Require().NoError(err)

	// Load configuration
	_, err = consul.Load(context.Background())
	s.Require().Error(err)
	s.Contains(err.Error(), "failed to decode consul value")
	s.Contains(err.Error(), "unexpected end of JSON input")
}

// TestLoad_WithInvalidJSON tests the Load method with an invalid JSON value
func (s *ConsulSourceTestSuite) TestLoad_WithInvalidJSON() {
	// Set up test data with invalid JSON
	key := "test/invalid-json"
	value := `{"foo": "bar", "baz": }` // Invalid JSON
	_, err := s.client.KV().Put(&api.KVPair{
		Key:   key,
		Value: []byte(value),
	}, nil)
	s.Require().NoError(err)

	// Create Consul source
	consul, err := NewConsul(key, &codec.JSONCodec{}, nil)
	s.Require().NoError(err)

	// Load configuration
	_, err = consul.Load(context.Background())
	s.Require().Error(err)
	s.Contains(err.Error(), "failed to decode consul value")
}

// TestLoad_WithInvalidYAML tests the Load method with an invalid YAML value
func (s *ConsulSourceTestSuite) TestLoad_WithInvalidYAML() {
	// Set up test data with invalid YAML
	key := "test/invalid-yaml"
	value := `
foo: bar
  baz: qux
    invalid: indentation
`
	_, err := s.client.KV().Put(&api.KVPair{
		Key:   key,
		Value: []byte(value),
	}, nil)
	s.Require().NoError(err)

	// Create Consul source
	consul, err := NewConsul(key, &codec.YAMLCodec{}, nil)
	s.Require().NoError(err)

	// Load configuration
	_, err = consul.Load(context.Background())
	s.Error(err)
	s.Contains(err.Error(), "failed to decode consul value")
}

// TestLoad_WithInvalidCasterValue tests the Load method with an invalid caster value
func (s *ConsulSourceTestSuite) TestLoad_WithInvalidCasterValue() {
	// Set up test data with invalid value for caster
	key := "test/invalid-caster"
	value := `not-a-number`
	_, err := s.client.KV().Put(&api.KVPair{
		Key:   key,
		Value: []byte(value),
	}, nil)
	s.Require().NoError(err)

	// Create Consul source with caster codec
	consul, err := NewConsul(key, codec.NewCaster(codec.CastTypeInt), nil)
	s.Require().NoError(err)

	// Load configuration
	_, err = consul.Load(context.Background())
	s.Require().Error(err)
	s.Contains(err.Error(), "failed to decode consul value")
}

// mockConsulKV is a mock implementation of the ConsulKV interface for testing
type mockConsulKV struct {
	err   error
	delay time.Duration
}

// Get is a mock implementation of the ConsulKV interface
func (m *mockConsulKV) Get(_ string, q *api.QueryOptions) (*api.KVPair, *api.QueryMeta, error) {
	if m.delay > 0 {
		ctx := q.Context()
		select {
		case <-time.After(m.delay):
			// Continue after delay
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		}
	}
	if m.err != nil {
		return nil, nil, m.err
	}
	return nil, nil, nil
}
