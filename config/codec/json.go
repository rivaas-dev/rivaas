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

import "encoding/json"

// TypeJSON is a constant representing the "json" encoding type.
const TypeJSON Type = "json"

// init registers the JSON encoding and decoding implementations with the codec package.
func init() {
	RegisterEncoder(TypeJSON, JSONCodec{})
	RegisterDecoder(TypeJSON, JSONCodec{})
}

// The JSONCodec struct implements the Encode and Decode methods to provide
// JSON serialization and deserialization functionality.
type JSONCodec struct{}

// Encode converts the provided value v into a JSON-encoded byte slice.
// It wraps the standard library's json.Marshal function.
func (JSONCodec) Encode(v any) ([]byte, error) {
	return json.Marshal(v)
}

// Decode unmarshalls the provided JSON-encoded byte slice into the value pointed to by v.
// It wraps the standard library's json.Unmarshal function.
func (JSONCodec) Decode(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
