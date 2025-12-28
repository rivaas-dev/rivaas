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

import "github.com/BurntSushi/toml"

// TypeTOML is a constant representing the "toml" encoding type.
const TypeTOML Type = "toml"

// init registers the TOML codec for encoding and decoding.
func init() {
	RegisterEncoder(TypeTOML, TOMLCodec{})
	RegisterDecoder(TypeTOML, TOMLCodec{})
}

// TOMLCodec is a struct that implements the Codec interface for TOML encoding and decoding.
type TOMLCodec struct{}

// Encode encodes the given value 'v' to a TOML-encoded byte slice.
func (TOMLCodec) Encode(v any) ([]byte, error) {
	return toml.Marshal(v)
}

// Decode decodes the TOML-encoded data into the value pointed to by v.
func (TOMLCodec) Decode(data []byte, v any) error {
	return toml.Unmarshal(data, v)
}
