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

import "fmt"

// Registry is a struct that holds the registered encoders and decoders.
// It provides methods to register and retrieve encoders and decoders.
type Registry struct {
	encoders map[Type]Encoder
	decoders map[Type]Decoder
}

var registry = &Registry{
	encoders: make(map[Type]Encoder),
	decoders: make(map[Type]Decoder),
}

// RegisterEncoder registers an encoder for the given type. The encoder can be
// used to encode values of the given type using the GetEncoder function.
func RegisterEncoder(name Type, encoder Encoder) {
	registry.encoders[name] = encoder
}

// RegisterDecoder registers a decoder for the given type. The decoder can be
// used to decode values of the given type using the GetDecoder function.
func RegisterDecoder(name Type, decoder Decoder) {
	registry.decoders[name] = decoder
}

// GetEncoder retrieves the registered encoder for the given type. If no encoder
// is registered for the given type, an error is returned.
func GetEncoder(name Type) (Encoder, error) {
	encoder, exists := registry.encoders[name]
	if !exists {
		return nil, fmt.Errorf("encoder not found for type: %s", name)
	}

	return encoder, nil
}

// GetDecoder retrieves the registered decoder for the given type. If no decoder
// is registered for the given type, an error is returned.
func GetDecoder(name Type) (Decoder, error) {
	decoder, exists := registry.decoders[name]
	if !exists {
		return nil, fmt.Errorf("decoder not found for type: %s", name)
	}

	return decoder, nil
}
