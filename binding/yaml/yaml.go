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

// Package yaml provides YAML binding support for the binding package.
//
// This package extends rivaas.dev/binding with YAML serialization support,
// using gopkg.in/yaml.v3 for parsing.
//
// Example:
//
//	type Config struct {
//	    Name    string `yaml:"name"`
//	    Port    int    `yaml:"port"`
//	    Debug   bool   `yaml:"debug"`
//	}
//
//	config, err := yaml.YAML[Config](body)
//	if err != nil {
//	    // handle error
//	}
package yaml

import (
	"bytes"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"

	"rivaas.dev/binding"
)

// Option configures YAML binding behavior.
type Option func(*config)

// config holds YAML-specific binding configuration.
type config struct {
	validator binding.Validator
	strict    bool
}

// WithValidator integrates external validation.
// The validator is called after successful binding.
func WithValidator(v binding.Validator) Option {
	return func(c *config) {
		c.validator = v
	}
}

// WithStrict enables strict YAML parsing.
// When enabled, unknown fields will cause an error.
func WithStrict() Option {
	return func(c *config) {
		c.strict = true
	}
}

func applyOptions(opts []Option) *config {
	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}

	return cfg
}

// YAML binds YAML bytes to type T.
//
// Example:
//
//	config, err := yaml.YAML[Config](body)
//
//	// With options
//	config, err := yaml.YAML[Config](body, yaml.WithStrict())
func YAML[T any](body []byte, opts ...Option) (T, error) {
	var result T
	cfg := applyOptions(opts)
	if err := bindYAMLBytes(&result, body, cfg); err != nil {
		return result, err
	}

	return result, nil
}

// YAMLReader binds YAML from an io.Reader to type T.
//
// Example:
//
//	config, err := yaml.YAMLReader[Config](r.Body)
func YAMLReader[T any](r io.Reader, opts ...Option) (T, error) {
	var result T
	cfg := applyOptions(opts)
	if err := bindYAMLReader(&result, r, cfg); err != nil {
		return result, err
	}

	return result, nil
}

// YAMLTo binds YAML bytes to out.
//
// Example:
//
//	var config Config
//	err := yaml.YAMLTo(body, &config)
func YAMLTo(body []byte, out any, opts ...Option) error {
	cfg := applyOptions(opts)
	return bindYAMLBytes(out, body, cfg)
}

// YAMLReaderTo binds YAML from an io.Reader to out.
//
// Example:
//
//	var config Config
//	err := yaml.YAMLReaderTo(r.Body, &config)
func YAMLReaderTo(r io.Reader, out any, opts ...Option) error {
	cfg := applyOptions(opts)
	return bindYAMLReader(out, r, cfg)
}

func bindYAMLBytes(out any, body []byte, cfg *config) error {
	if cfg.strict {
		decoder := yaml.NewDecoder(bytes.NewReader(body))
		decoder.KnownFields(true)
		if err := decoder.Decode(out); err != nil {
			return err
		}
	} else {
		if err := yaml.Unmarshal(body, out); err != nil {
			return err
		}
	}

	// Run validator if configured
	if cfg.validator != nil {
		if err := cfg.validator.Validate(out); err != nil {
			return &binding.BindError{
				Field:  "",
				Source: binding.SourceYAML,
				Reason: fmt.Sprintf("validation failed: %v", err),
				Err:    err,
			}
		}
	}

	return nil
}

func bindYAMLReader(out any, r io.Reader, cfg *config) error {
	decoder := yaml.NewDecoder(r)
	if cfg.strict {
		decoder.KnownFields(true)
	}

	if err := decoder.Decode(out); err != nil {
		return err
	}

	// Run validator if configured
	if cfg.validator != nil {
		if err := cfg.validator.Validate(out); err != nil {
			return &binding.BindError{
				Field:  "",
				Source: binding.SourceYAML,
				Reason: fmt.Sprintf("validation failed: %v", err),
				Err:    err,
			}
		}
	}

	return nil
}

// sourceGetter is a marker type for YAML body source.
type sourceGetter struct {
	body   []byte
	cfg    *config
	reader io.Reader
}

func (s *sourceGetter) Get(key string) string      { return "" }
func (s *sourceGetter) GetAll(key string) []string { return nil }
func (s *sourceGetter) Has(key string) bool        { return false }

// FromYAML returns a binding.Option that specifies YAML body as a binding source.
// This can be used with binding.Bind for multi-source binding.
//
// Note: When using FromYAML, the YAML binding is handled specially in the
// multi-source binding flow. See binding.Bind documentation.
//
// Example:
//
//	req, err := binding.Bind[Request](
//	    binding.FromQuery(r.URL.Query()),
//	    yaml.FromYAML(body),
//	)
func FromYAML(body []byte, opts ...Option) binding.Option {
	cfg := applyOptions(opts)
	return binding.FromGetter(&sourceGetter{body: body, cfg: cfg}, binding.TagYAML)
}

// FromYAMLReader returns a binding.Option that specifies YAML from io.Reader as a binding source.
//
// Example:
//
//	req, err := binding.Bind[Request](
//	    yaml.FromYAMLReader(r.Body),
//	)
func FromYAMLReader(r io.Reader, opts ...Option) binding.Option {
	cfg := applyOptions(opts)
	return binding.FromGetter(&sourceGetter{reader: r, cfg: cfg}, binding.TagYAML)
}
