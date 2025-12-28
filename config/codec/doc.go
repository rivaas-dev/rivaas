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

// Package codec provides encoding and decoding functionality for configuration data.
//
// The codec package defines [Encoder] and [Decoder] interfaces for converting
// configuration data between different formats (JSON, YAML, TOML, etc.) and Go types.
// It includes a registry system for registering and retrieving codec implementations.
//
// # Built-in Codecs
//
// The package includes built-in support for common formats:
//
//   - JSON: Standard JSON encoding/decoding
//   - YAML: YAML encoding/decoding
//   - TOML: TOML encoding/decoding
//   - EnvVar: Environment variable format
//
// # Custom Codecs
//
// Register custom codecs using [RegisterEncoder] and [RegisterDecoder]:
//
//	type MyCodec struct{}
//
//	func (c MyCodec) Encode(v any) ([]byte, error) {
//	    // Custom encoding logic
//	    return data, nil
//	}
//
//	func (c MyCodec) Decode(data []byte, v any) error {
//	    // Custom decoding logic
//	    return nil
//	}
//
//	codec.RegisterEncoder(codec.Type("myformat"), MyCodec{})
//	codec.RegisterDecoder(codec.Type("myformat"), MyCodec{})
//
// # Type Casting
//
// The package includes caster codecs for automatic type conversion:
//
//	decoder, _ := codec.GetDecoder(codec.TypeCasterInt)
//	var value any
//	decoder.Decode([]byte("42"), &value)  // value is int(42)
//
// Supported caster types include bool, string, int variants, uint variants,
// float variants, time.Time, and time.Duration.
package codec
