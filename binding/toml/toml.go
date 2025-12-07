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

// Package toml provides TOML binding support for the binding package.
//
// This package extends rivaas.dev/binding with TOML serialization support,
// using github.com/BurntSushi/toml for parsing.
//
// Example:
//
//	type Config struct {
//	    Title   string `toml:"title"`
//	    Port    int    `toml:"port"`
//	    Debug   bool   `toml:"debug"`
//	}
//
//	config, err := toml.TOML[Config](body)
//	if err != nil {
//	    // handle error
//	}
package toml

import (
	"bytes"
	"fmt"
	"io"

	"github.com/BurntSushi/toml"

	"rivaas.dev/binding"
)

// Option configures TOML binding behavior.
type Option func(*config)

// config holds TOML-specific binding configuration.
type config struct {
	validator binding.Validator
}

// WithValidator integrates external validation.
// The validator is called after successful binding.
func WithValidator(v binding.Validator) Option {
	return func(c *config) {
		c.validator = v
	}
}

func applyOptions(opts []Option) *config {
	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

// Metadata holds information about undecoded keys.
type Metadata = toml.MetaData

// TOML binds TOML bytes to type T.
//
// Example:
//
//	config, err := toml.TOML[Config](body)
func TOML[T any](body []byte, opts ...Option) (T, error) {
	var result T
	cfg := applyOptions(opts)
	if err := bindTOMLBytes(&result, body, cfg); err != nil {
		return result, err
	}
	return result, nil
}

// TOMLWithMetadata binds TOML bytes to type T and returns metadata.
// The metadata contains information about which keys were decoded.
//
// Example:
//
//	config, meta, err := toml.TOMLWithMetadata[Config](body)
//	if len(meta.Undecoded()) > 0 {
//	    log.Printf("Unknown keys: %v", meta.Undecoded())
//	}
func TOMLWithMetadata[T any](body []byte, opts ...Option) (T, Metadata, error) {
	var result T
	cfg := applyOptions(opts)
	meta, err := bindTOMLBytesWithMeta(&result, body, cfg)
	if err != nil {
		return result, meta, err
	}
	return result, meta, nil
}

// TOMLReader binds TOML from an io.Reader to type T.
//
// Example:
//
//	config, err := toml.TOMLReader[Config](r.Body)
func TOMLReader[T any](r io.Reader, opts ...Option) (T, error) {
	var result T
	cfg := applyOptions(opts)
	if err := bindTOMLReader(&result, r, cfg); err != nil {
		return result, err
	}
	return result, nil
}

// TOMLTo binds TOML bytes to out.
//
// Example:
//
//	var config Config
//	err := toml.TOMLTo(body, &config)
func TOMLTo(body []byte, out any, opts ...Option) error {
	cfg := applyOptions(opts)
	return bindTOMLBytes(out, body, cfg)
}

// TOMLReaderTo binds TOML from an io.Reader to out.
//
// Example:
//
//	var config Config
//	err := toml.TOMLReaderTo(r.Body, &config)
func TOMLReaderTo(r io.Reader, out any, opts ...Option) error {
	cfg := applyOptions(opts)
	return bindTOMLReader(out, r, cfg)
}

func bindTOMLBytes(out any, body []byte, cfg *config) error {
	if _, err := toml.Decode(string(body), out); err != nil {
		return err
	}

	// Run validator if configured
	if cfg.validator != nil {
		if err := cfg.validator.Validate(out); err != nil {
			return &binding.BindError{
				Field:  "",
				Source: binding.SourceTOML,
				Reason: fmt.Sprintf("validation failed: %v", err),
				Err:    err,
			}
		}
	}

	return nil
}

func bindTOMLBytesWithMeta(out any, body []byte, cfg *config) (Metadata, error) {
	meta, err := toml.Decode(string(body), out)
	if err != nil {
		return meta, err
	}

	// Run validator if configured
	if cfg.validator != nil {
		if err := cfg.validator.Validate(out); err != nil {
			return meta, &binding.BindError{
				Field:  "",
				Source: binding.SourceTOML,
				Reason: fmt.Sprintf("validation failed: %v", err),
				Err:    err,
			}
		}
	}

	return meta, nil
}

func bindTOMLReader(out any, r io.Reader, cfg *config) error {
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(r); err != nil {
		return err
	}

	return bindTOMLBytes(out, buf.Bytes(), cfg)
}

// sourceGetter is a marker type for TOML body source.
type sourceGetter struct {
	body   []byte
	cfg    *config
	reader io.Reader
}

func (s *sourceGetter) Get(key string) string      { return "" }
func (s *sourceGetter) GetAll(key string) []string { return nil }
func (s *sourceGetter) Has(key string) bool        { return false }

// FromTOML returns a binding.Option that specifies TOML body as a binding source.
// This can be used with binding.Bind for multi-source binding.
//
// Example:
//
//	req, err := binding.Bind[Request](
//	    binding.FromQuery(r.URL.Query()),
//	    toml.FromTOML(body),
//	)
func FromTOML(body []byte, opts ...Option) binding.Option {
	cfg := applyOptions(opts)
	return binding.FromGetter(&sourceGetter{body: body, cfg: cfg}, binding.TagTOML)
}

// FromTOMLReader returns a binding.Option that specifies TOML from io.Reader as a binding source.
//
// Example:
//
//	req, err := binding.Bind[Request](
//	    toml.FromTOMLReader(r.Body),
//	)
func FromTOMLReader(r io.Reader, opts ...Option) binding.Option {
	cfg := applyOptions(opts)
	return binding.FromGetter(&sourceGetter{reader: r, cfg: cfg}, binding.TagTOML)
}
