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

// Package codec provides functionality for encoding and decoding data.
package codec

import "github.com/goccy/go-yaml"

// TypeYAML is a constant representing the "yaml" encoding type.
const TypeYAML Type = "yaml"

// init registers the YAML codec for encoding and decoding.
func init() {
	RegisterEncoder(TypeYAML, YAMLCodec{})
	RegisterDecoder(TypeYAML, YAMLCodec{})
}

// YAMLCodec is a struct that implements the Codec interface for YAML encoding and decoding.
type YAMLCodec struct{}

// Encode encodes the given value 'v' to a YAML-encoded byte slice.
func (YAMLCodec) Encode(v any) ([]byte, error) {
	return yaml.Marshal(v)
}

// Decode decodes the YAML-encoded data into the value pointed to by v.
func (YAMLCodec) Decode(data []byte, v any) error {
	return yaml.Unmarshal(data, v)
}
