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

package config

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"dario.cat/mergo"
	"github.com/go-viper/mapstructure/v2"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/spf13/cast"

	"rivaas.dev/config/codec"
	"rivaas.dev/config/dumper"
	"rivaas.dev/config/source"
)

// Option is a functional option that can be used to configure a Config instance.
type Option func(c *Config) error

// Config manages configuration data loaded from multiple sources.
// It provides thread-safe access to configuration values and supports
// binding to structs, validation, and dumping to files.
//
// Config is safe for concurrent use by multiple goroutines.
type Config struct {
	values             *map[string]any
	sources            []Source
	dumpers            []Dumper
	binding            any
	tagName            string // Custom struct tag name (default: "config")
	mu                 sync.RWMutex
	jsonSchemaCompiled *jsonschema.Schema
	customValidators   []func(map[string]any) error
	// decoderConfig holds the cached decoder configuration for struct binding
	decoderConfig *mapstructure.DecoderConfig
	decoderOnce   sync.Once
}

// WithSource adds a source to the configuration loader.
func WithSource(loader Source) Option {
	return func(c *Config) error {
		if loader == nil {
			return errors.New("source cannot be nil")
		}
		c.sources = append(c.sources, loader)
		return nil
	}
}

// WithFileDumper returns an Option that configures the Config instance to dump configuration data to a file.
// The format is automatically detected from the file extension (.yaml, .yml, .json, .toml).
// For files without extensions or custom formats, use WithFileDumperAs instead.
//
// Paths support environment variable expansion using ${VAR} or $VAR syntax.
// Example: "${LOG_DIR}/config.yaml" expands to "/var/log/config.yaml" when LOG_DIR=/var/log
//
// Example:
//
//	cfg := config.MustNew(
//	    config.WithFile("config.yaml"),
//	    config.WithFileDumper("output.yaml"),  // Auto-detects YAML
//	)
func WithFileDumper(path string) Option {
	return func(c *Config) error {
		path = os.ExpandEnv(path)

		format, err := detectFormat(path)
		if err != nil {
			return NewError("file-dumper", "detect-format", err)
		}

		encoder, err := codec.GetEncoder(format)
		if err != nil {
			return NewError("file-dumper", "get-encoder", err)
		}

		c.dumpers = append(c.dumpers, dumper.NewFile(path, encoder))
		return nil
	}
}

// WithDumper adds a dumper to the configuration loader.
func WithDumper(dumper Dumper) Option {
	return func(c *Config) error {
		if dumper == nil {
			return errors.New("dumper cannot be nil")
		}
		c.dumpers = append(c.dumpers, dumper)
		return nil
	}
}

// WithFile returns an Option that configures the Config instance to load configuration data from a file.
// The format is automatically detected from the file extension (.yaml, .yml, .json, .toml).
// For files without extensions or custom formats, use WithFileAs instead.
//
// Paths support environment variable expansion using ${VAR} or $VAR syntax.
// Example: "${CONFIG_DIR}/app.yaml" expands to "/etc/myapp/app.yaml" when CONFIG_DIR=/etc/myapp
//
// Example:
//
//	cfg := config.MustNew(
//	    config.WithFile("config.yaml"),     // Automatically detects YAML
//	    config.WithFile("override.json"),   // Automatically detects JSON
//	)
func WithFile(path string) Option {
	return func(c *Config) error {
		path = os.ExpandEnv(path)

		format, err := detectFormat(path)
		if err != nil {
			return NewError("file-source", "detect-format", err)
		}

		decoder, err := codec.GetDecoder(format)
		if err != nil {
			return NewError("file-source", "get-decoder", err)
		}

		c.sources = append(c.sources, source.NewFile(path, decoder))
		return nil
	}
}

// WithEnv returns an Option that configures the Config instance to load configuration data from environment variables.
// The prefix parameter specifies the prefix for the environment variables to be loaded.
// Environment variables are converted to lowercase and underscores create nested structures.
//
// Example:
//
//	cfg := config.MustNew(
//	    config.WithFile("config.yaml"),
//	    config.WithEnv("APP_"),  // Loads APP_SERVER_PORT as server.port
//	)
func WithEnv(prefix string) Option {
	return func(c *Config) error {
		c.sources = append(c.sources, source.NewOSEnvVar(prefix))
		return nil
	}
}

// WithConsul returns an Option that configures the Config instance to load configuration data from a Consul server.
// The format is automatically detected from the path extension.
// For custom formats, use WithConsulAs instead.
//
// If CONSUL_HTTP_ADDR is not set, this option is silently skipped, allowing
// development without Consul while requiring it in production environments.
//
// Paths support environment variable expansion using ${VAR} or $VAR syntax.
// Example: "${APP_ENV}/service.yaml" expands to "production/service.yaml" when APP_ENV=production
//
// Required environment variables (production only):
//   - CONSUL_HTTP_ADDR: The address of the Consul server (e.g., "http://localhost:8500")
//   - CONSUL_HTTP_TOKEN: The access token for authentication with Consul (optional)
//
// Example:
//
//	cfg := config.MustNew(
//	    config.WithConsul("production/service.yaml"),  // Auto-detects YAML, skipped without CONSUL_HTTP_ADDR
//	)
func WithConsul(path string) Option {
	return func(c *Config) error {
		// Silently skip if Consul is not configured
		if os.Getenv("CONSUL_HTTP_ADDR") == "" {
			return nil
		}

		path = os.ExpandEnv(path)

		format, err := detectFormat(path)
		if err != nil {
			return NewError("consul-source", "detect-format", err)
		}

		decoder, err := codec.GetDecoder(format)
		if err != nil {
			return NewError("consul-source", "get-decoder", err)
		}

		l, err := source.NewConsul(path, decoder, nil)
		if err != nil {
			return NewError("consul-source", "create-client", err)
		}

		c.sources = append(c.sources, l)
		return nil
	}
}

// WithFileAs returns an Option that configures the Config instance to load configuration data from a file with explicit format.
// Use this when the file doesn't have an extension or when you need to override the format detection.
//
// Paths support environment variable expansion using ${VAR} or $VAR syntax.
// Example: "${CONFIG_DIR}/app" expands to "/etc/myapp/app" when CONFIG_DIR=/etc/myapp
//
// Example:
//
//	cfg := config.MustNew(
//	    config.WithFileAs("config", codec.TypeYAML),      // No extension, specify YAML
//	    config.WithFileAs("config.dat", codec.TypeJSON),  // Wrong extension, specify JSON
//	)
func WithFileAs(path string, codecType codec.Type) Option {
	return func(c *Config) error {
		path = os.ExpandEnv(path)

		decoder, err := codec.GetDecoder(codecType)
		if err != nil {
			return NewError("file-source", "get-decoder", err)
		}

		c.sources = append(c.sources, source.NewFile(path, decoder))
		return nil
	}
}

// WithConsulAs returns an Option that configures the Config instance to load configuration data from a Consul server with explicit format.
// Use this when you need to override the format detection.
//
// If CONSUL_HTTP_ADDR is not set, this option is silently skipped, allowing
// development without Consul while requiring it in production environments.
//
// Paths support environment variable expansion using ${VAR} or $VAR syntax.
// Example: "${APP_ENV}/service" expands to "production/service" when APP_ENV=production
//
// Required environment variables (production only):
//   - CONSUL_HTTP_ADDR: The address of the Consul server (e.g., "http://localhost:8500")
//   - CONSUL_HTTP_TOKEN: The access token for authentication with Consul (optional)
//
// Example:
//
//	cfg := config.MustNew(
//	    config.WithConsulAs("production/service", codec.TypeJSON),
//	)
func WithConsulAs(path string, codecType codec.Type) Option {
	return func(c *Config) error {
		// Silently skip if Consul is not configured
		if os.Getenv("CONSUL_HTTP_ADDR") == "" {
			return nil
		}

		path = os.ExpandEnv(path)

		decoder, err := codec.GetDecoder(codecType)
		if err != nil {
			return NewError("consul-source", "get-decoder", err)
		}

		l, err := source.NewConsul(path, decoder, nil)
		if err != nil {
			return NewError("consul-source", "create-client", err)
		}

		c.sources = append(c.sources, l)
		return nil
	}
}

// WithContent returns an Option that configures the Config instance to load configuration data from a byte slice.
// The codecType parameter specifies the format of the data (e.g., codec.TypeJSON, codec.TypeYAML).
//
// Example:
//
//	yamlContent := []byte("server:\n  port: 8080")
//	cfg := config.MustNew(
//	    config.WithContent(yamlContent, codec.TypeYAML),
//	)
func WithContent(data []byte, codecType codec.Type) Option {
	return func(c *Config) error {
		decoder, err := codec.GetDecoder(codecType)
		if err != nil {
			return NewError("content-source", "get-decoder", err)
		}

		c.sources = append(c.sources, source.NewFileContent(data, decoder))
		return nil
	}
}

// WithFileDumperAs returns an Option that configures the Config instance to dump configuration data to a file with explicit format.
// Use this when the file doesn't have an extension or when you need to override the format detection.
//
// Paths support environment variable expansion using ${VAR} or $VAR syntax.
// Example: "${OUTPUT_DIR}/config" expands to "/tmp/config" when OUTPUT_DIR=/tmp
//
// Example:
//
//	cfg := config.MustNew(
//	    config.WithFile("config.yaml"),
//	    config.WithFileDumperAs("output", codec.TypeYAML),  // No extension, specify YAML
//	)
func WithFileDumperAs(path string, codecType codec.Type) Option {
	return func(c *Config) error {
		path = os.ExpandEnv(path)

		encoder, err := codec.GetEncoder(codecType)
		if err != nil {
			return NewError("file-dumper", "get-encoder", err)
		}

		c.dumpers = append(c.dumpers, dumper.NewFile(path, encoder))
		return nil
	}
}

// WithBinding returns an Option that configures the Config instance to bind configuration data to a struct.
func WithBinding(v any) Option {
	return func(c *Config) error {
		if v == nil {
			return errors.New("binding target cannot be nil")
		}
		if reflect.TypeOf(v).Kind() != reflect.Ptr {
			return errors.New("binding target must be a pointer")
		}
		c.binding = v
		return nil
	}
}

// WithTag sets a custom struct tag name for binding (default: "config").
// This allows you to use a different tag name if "config" conflicts with other libraries.
//
// Example:
//
//	type Config struct {
//	    Port int `cfg:"port"`  // Using custom tag
//	}
//
//	cfg := config.MustNew(
//	    config.WithFile("config.yaml"),
//	    config.WithBinding(&appConfig),
//	    config.WithTag("cfg"),  // Use "cfg" instead of "config"
//	)
func WithTag(tagName string) Option {
	return func(c *Config) error {
		if tagName == "" {
			return errors.New("tag name cannot be empty")
		}
		c.tagName = tagName
		return nil
	}
}

// WithJSONSchema adds a JSON Schema for validation.
func WithJSONSchema(schema []byte) Option {
	return func(c *Config) error {
		// Use a unique schema name to avoid caching issues
		//nolint:gosec // rand.Int() is used for a unique schema name, not security sensitive
		schemaName := fmt.Sprintf("inline_%d.json", rand.Int())
		compiler := jsonschema.NewCompiler()

		jsonSchema, err := jsonschema.UnmarshalJSON(bytes.NewReader(schema))
		if err != nil {
			return err
		}

		if err = compiler.AddResource(schemaName, jsonSchema); err != nil {
			return err
		}
		s, err := compiler.Compile(schemaName)
		if err != nil {
			return err
		}
		c.jsonSchemaCompiled = s
		return nil
	}
}

// WithValidator adds a custom validation function.
func WithValidator(fn func(map[string]any) error) Option {
	return func(c *Config) error {
		c.customValidators = append(c.customValidators, fn)
		return nil
	}
}

// New creates a new Config instance with the provided options.
// It iterates through the options and applies each one to the Config instance.
// If any of the options return an error, the errors are collected and returned.
// Returns a partially initialized Config along with any errors encountered.
func New(options ...Option) (*Config, error) {
	var errs error
	c := &Config{
		values:  &map[string]any{},
		sources: []Source{},
		tagName: "config", // Default tag name
	}

	for _, option := range options {
		if option == nil {
			continue // Skip nil options
		}
		err := option(c)
		if err != nil {
			errs = errors.Join(errs, err)
		}
	}

	return c, errs //nolint:nilnil // Returning partial config with error is intentional
}

// MustNew creates a new Config instance with the provided options.
// It panics if any option returns an error.
// Use this in main() or initialization code where panic is acceptable.
// For cases where error handling is needed, use New() instead.
func MustNew(options ...Option) *Config {
	cfg, err := New(options...)
	if err != nil {
		panic(fmt.Sprintf("config: failed to create config: %v", err))
	}
	return cfg
}

// Validator is an interface for structs that can validate their own configuration.
type Validator interface {
	Validate() error
}

// applyDefaults applies default values from struct tags to a struct.
// It walks through the struct fields and sets defaults for fields that have the 'default' tag
// and are currently zero-valued.
func applyDefaults(target interface{}) error {
	val := reflect.ValueOf(target)
	if val.Kind() != reflect.Ptr {
		return fmt.Errorf("target must be a pointer")
	}

	val = val.Elem()
	if val.Kind() != reflect.Struct {
		return fmt.Errorf("target must be a pointer to a struct")
	}

	return setDefaults(val)
}

// setDefaults recursively sets default values on a struct.
func setDefaults(val reflect.Value) error {
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// Skip unexported fields
		if !field.CanSet() {
			continue
		}

		// Handle nested structs
		if field.Kind() == reflect.Struct {
			if err := setDefaults(field); err != nil {
				return err
			}
			continue
		}

		// Check if field has a default tag
		defaultTag := fieldType.Tag.Get("default")
		if defaultTag == "" {
			continue
		}

		// Only set default if field is zero-valued
		if !isZeroValue(field) {
			continue
		}

		// Set the default value based on field type
		if err := setDefaultValue(field, defaultTag); err != nil {
			return fmt.Errorf("failed to set default for field %s: %w", fieldType.Name, err)
		}
	}

	return nil
}

// isZeroValue checks if a reflect.Value is the zero value for its type.
func isZeroValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	}
	return false
}

// setDefaultValue sets a default value on a field based on its type.
func setDefaultValue(field reflect.Value, defaultVal string) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(defaultVal)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		// Special handling for time.Duration
		if field.Type() == reflect.TypeOf(time.Duration(0)) {
			d, err := time.ParseDuration(defaultVal)
			if err != nil {
				return err
			}
			field.SetInt(int64(d))
		} else {
			i, err := cast.ToInt64E(defaultVal)
			if err != nil {
				return err
			}
			field.SetInt(i)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u, err := cast.ToUint64E(defaultVal)
		if err != nil {
			return err
		}
		field.SetUint(u)
	case reflect.Float32, reflect.Float64:
		f, err := cast.ToFloat64E(defaultVal)
		if err != nil {
			return err
		}
		field.SetFloat(f)
	case reflect.Bool:
		b, err := cast.ToBoolE(defaultVal)
		if err != nil {
			return err
		}
		field.SetBool(b)
	default:
		return fmt.Errorf("unsupported type for default tag: %s", field.Kind())
	}
	return nil
}

// getDecoderConfig returns a cached decoder configuration to reduce reflection overhead.
func (c *Config) getDecoderConfig() *mapstructure.DecoderConfig {
	c.decoderOnce.Do(func() {
		tagName := c.tagName
		if tagName == "" {
			tagName = "config" // Fallback to default
		}
		c.decoderConfig = &mapstructure.DecoderConfig{
			TagName:          tagName,
			Squash:           true,
			WeaklyTypedInput: true,
			DecodeHook: mapstructure.ComposeDecodeHookFunc(
				mapstructure.StringToTimeDurationHookFunc(),
				mapstructure.StringToSliceHookFunc(","),
				mapstructure.StringToTimeHookFunc(time.RFC3339),
				mapstructure.StringToURLHookFunc(),
			),
		}
	})
	return c.decoderConfig
}

// normalizeMapKeys recursively converts all map keys to lowercase for case-insensitive merging
func normalizeMapKeys(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	normalized := make(map[string]any)
	for k, v := range m {
		lowerKey := strings.ToLower(k)
		if nestedMap, ok := v.(map[string]any); ok {
			normalized[lowerKey] = normalizeMapKeys(nestedMap)
		} else {
			normalized[lowerKey] = v
		}
	}
	return normalized
}

// loadSourcesSequential loads configuration data from all sources sequentially to avoid race conditions.
func (c *Config) loadSourcesSequential(ctx context.Context) (map[string]any, error) {
	if len(c.sources) == 0 {
		return make(map[string]any), nil
	}

	// Merge to maintain precedence
	newValues := make(map[string]any)
	for i, src := range c.sources {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		conf, err := src.Load(ctx)
		if err != nil {
			return nil, NewError(fmt.Sprintf("source[%d]", i), "load", err)
		}

		// Ensure we always have a valid map, even if source returns nil
		if conf == nil {
			conf = make(map[string]any)
		}

		// Normalize keys to lowercase for case-insensitive merging
		normalizedConf := normalizeMapKeys(conf)

		// Use mergo to merge configuration maps with override behavior
		if err = mergo.Map(&newValues, normalizedConf, mergo.WithOverride); err != nil {
			return nil, NewError(fmt.Sprintf("source[%d]", i), "merge", err)
		}
	}

	return newValues, nil
}

// Load loads configuration data from the registered sources and merges it into the internal values map.
// The method validates the configuration data before atomically updating the internal state.
// Load is safe to call concurrently.
//
// Errors:
//   - Returns error if ctx is nil
//   - Returns [ConfigError] if any source fails to load
//   - Returns [ConfigError] if JSON schema validation fails
//   - Returns [ConfigError] if custom validators fail
//   - Returns [ConfigError] if binding or struct validation fails
func (c *Config) Load(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context cannot be nil")
	}

	newValues, err := c.loadSourcesSequential(ctx)
	if err != nil {
		return err
	}

	// Ensure newValues is never nil
	if newValues == nil {
		newValues = make(map[string]any)
	}

	if c.jsonSchemaCompiled != nil {
		if err = c.jsonSchemaCompiled.Validate(newValues); err != nil {
			return NewError("json-schema", "validate", err)
		}
	}

	// Custom function validators
	for i, fn := range c.customValidators {
		if fn == nil {
			continue // Skip nil validators
		}
		var validatorErr error
		func() {
			defer func() {
				if r := recover(); r != nil {
					validatorErr = fmt.Errorf("validator panic: %v", r)
				}
			}()
			validatorErr = fn(newValues)
		}()
		if validatorErr != nil {
			return NewError(fmt.Sprintf("custom-validator[%d]", i), "validate", validatorErr)
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.binding != nil {
		// Validate binding without modifying shared state
		if err = c.bindAndValidate(newValues); err != nil {
			return NewError("binding", "validate", err)
		}
		// Now safely update the actual binding struct
		if err = c.bind(&newValues); err != nil {
			return NewError("binding", "bind", err)
		}
	}

	c.values = &newValues

	return nil
}

// MustLoad loads configuration or panics on error.
// This is a convenience wrapper around Load for use cases where configuration
// loading failure should halt the program, typically in main() or init().
//
// For error handling, use Load instead.
//
// Example:
//
//	cfg := config.MustNew(
//	    config.WithFile("config.yaml"),
//	)
//	cfg.MustLoad(context.Background())
func (c *Config) MustLoad(ctx context.Context) {
	if err := c.Load(ctx); err != nil {
		panic(err)
	}
}

// Dump writes the current configuration values to the registered dumpers.
//
// Errors:
//   - Returns error if ctx is nil
//   - Returns error if any dumper fails to write the configuration
func (c *Config) Dump(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context cannot be nil")
	}

	// Get a copy of the values to avoid holding locks during dumper calls
	var valuesCopy map[string]any
	func() {
		c.mu.RLock()
		defer c.mu.RUnlock()
		if c.values != nil {
			// Use shallow copy for better performance
			valuesCopy = make(map[string]any, len(*c.values))
			for k, v := range *c.values {
				valuesCopy[k] = v
			}
		} else {
			valuesCopy = make(map[string]any)
		}
	}()

	for _, d := range c.dumpers {
		if err := d.Dump(ctx, &valuesCopy); err != nil {
			return err
		}
	}

	return nil
}

// MustDump writes configuration to dumpers or panics on error.
// This is a convenience wrapper around Dump for use cases where dump
// failure should halt the program.
//
// For error handling, use Dump instead.
//
// Example:
//
//	cfg := config.MustNew(
//	    config.WithDumper(myDumper),
//	)
//	cfg.MustDump(context.Background())
func (c *Config) MustDump(ctx context.Context) {
	if err := c.Dump(ctx); err != nil {
		panic(err)
	}
}

func (c *Config) bind(values *map[string]any) error {
	// Get the decoder config and set the result target
	config := c.getDecoderConfig()
	config.Result = c.binding

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return fmt.Errorf("failed to create decoder: %w", err)
	}

	if err = decoder.Decode(values); err != nil {
		return fmt.Errorf("failed to decode configuration: %w", err)
	}

	// Apply default values from struct tags
	if err = applyDefaults(c.binding); err != nil {
		return fmt.Errorf("failed to apply defaults: %w", err)
	}

	return nil
}

// bindAndValidate performs binding and validation on the provided values without modifying shared state.
// This method is used during Load to validate configuration before atomically updating c.values.
func (c *Config) bindAndValidate(values map[string]any) error {
	// Create a temporary copy of the binding struct to avoid race conditions
	// when multiple goroutines call Load() concurrently
	bindingType := reflect.TypeOf(c.binding)
	if bindingType.Kind() == reflect.Ptr {
		bindingType = bindingType.Elem()
	}
	tempBinding := reflect.New(bindingType).Interface()

	// Get the decoder config and set the result target
	config := c.getDecoderConfig()
	config.Result = tempBinding

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return fmt.Errorf("failed to create decoder: %w", err)
	}

	if err = decoder.Decode(&values); err != nil {
		return fmt.Errorf("failed to decode configuration: %w", err)
	}

	// Apply default values from struct tags
	if err = applyDefaults(tempBinding); err != nil {
		return fmt.Errorf("failed to apply defaults: %w", err)
	}

	// Run validation if the binding implements Validator interface
	if v, ok := tempBinding.(Validator); ok {
		if err = v.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// Values returns a pointer to the internal values map of the Config instance.
// The map is protected by a read lock, which is acquired and released within this method.
// This method is used to safely access the internal values map.
func (c *Config) Values() *map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.values == nil {
		emptyMap := make(map[string]any)
		return &emptyMap
	}

	return c.values
}

// getValueFromMap retrieves the value associated with the given path from the internal values map.
// The path is a dot-separated string that represents the nested structure of the map.
// If the path is valid and the final value is found, it is returned. Otherwise, nil is returned.
// Keys are case-insensitive since they are stored in lowercase.
func (c *Config) getValueFromMap(path string) any {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.values == nil {
		return nil
	}

	// Work with a copy of the current map to avoid race conditions during traversal
	current := *c.values

	// Normalize the path to lowercase for case-insensitive lookup
	normalizedPath := strings.ToLower(path)

	// 1. Check for direct key match first
	if val, ok := current[normalizedPath]; ok {
		return val
	}

	// 2. Fallback to dot notation traversal
	segments := strings.Split(normalizedPath, ".")
	for i, segment := range segments {
		if currentMap, ok := current[segment]; ok {
			if i == len(segments)-1 {
				return currentMap
			}
			if nestedMap, isMap := currentMap.(map[string]any); isMap {
				current = nestedMap
			} else {
				return nil
			}
		} else {
			return nil
		}
	}
	return nil
}

// Get returns the value associated with the given key as an any type.
// If the key is not found, it returns nil.
func (c *Config) Get(key string) any {
	if c == nil {
		return nil
	}
	if key == "" {
		return nil
	}
	return c.getValueFromMap(key)
}

// String returns the value associated with the given key as a string.
// If the value is not found or cannot be converted to a string, an empty string is returned.
//
// Example:
//
//	host := cfg.String("server.host")
func (c *Config) String(key string) string {
	if c == nil {
		return ""
	}
	return cast.ToString(c.Get(key))
}

// Int returns the value associated with the given key as an int.
// If the value is not found or cannot be converted to an int, 0 is returned.
//
// Example:
//
//	port := cfg.Int("server.port")
func (c *Config) Int(key string) int {
	if c == nil {
		return 0
	}
	return cast.ToInt(c.Get(key))
}

// Int64 returns the value associated with the given key as an int64.
// If the value is not found or cannot be converted to an int64, 0 is returned.
//
// Example:
//
//	maxSize := cfg.Int64("max_size")
func (c *Config) Int64(key string) int64 {
	if c == nil {
		return 0
	}
	return cast.ToInt64(c.Get(key))
}

// Float64 returns the value associated with the given key as a float64.
// If the value is not found or cannot be converted to a float64, 0.0 is returned.
//
// Example:
//
//	rate := cfg.Float64("rate")
func (c *Config) Float64(key string) float64 {
	if c == nil {
		return 0.0
	}
	return cast.ToFloat64(c.Get(key))
}

// Bool returns the value associated with the given key as a boolean.
// If the value is not found or cannot be converted to a boolean, false is returned.
//
// Example:
//
//	debug := cfg.Bool("debug")
func (c *Config) Bool(key string) bool {
	if c == nil {
		return false
	}
	return cast.ToBool(c.Get(key))
}

// Duration returns the value associated with the given key as a time.Duration.
// If the value is not found or cannot be converted to a time.Duration, the zero value is returned.
//
// Example:
//
//	timeout := cfg.Duration("timeout")
func (c *Config) Duration(key string) time.Duration {
	if c == nil {
		return 0
	}
	return cast.ToDuration(c.Get(key))
}

// Time returns the value associated with the given key as a time.Time.
// If the value is not found or cannot be converted to a time.Time, the zero value is returned.
//
// Example:
//
//	startTime := cfg.Time("start_time")
func (c *Config) Time(key string) time.Time {
	if c == nil {
		return time.Time{}
	}
	return cast.ToTime(c.Get(key))
}

// StringSlice returns the value associated with the given key as a slice of strings.
// If the value is not found or cannot be converted to a slice of strings, an empty slice is returned.
//
// Example:
//
//	tags := cfg.StringSlice("tags")
func (c *Config) StringSlice(key string) []string {
	if c == nil {
		return []string{}
	}
	return cast.ToStringSlice(c.Get(key))
}

// IntSlice returns the value associated with the given key as a slice of integers.
// If the value is not found or cannot be converted to a slice of integers, an empty slice is returned.
//
// Example:
//
//	ports := cfg.IntSlice("ports")
func (c *Config) IntSlice(key string) []int {
	if c == nil {
		return []int{}
	}
	return cast.ToIntSlice(c.Get(key))
}

// StringMap returns the value associated with the given key as a map[string]any.
// If the value is not found or cannot be converted to a map[string]any, an empty map is returned.
//
// Example:
//
//	metadata := cfg.StringMap("metadata")
func (c *Config) StringMap(key string) map[string]any {
	if c == nil {
		return map[string]any{}
	}
	return cast.ToStringMap(c.Get(key))
}

// StringOr returns the value associated with the given key as a string, or the default value if not found.
//
// Example:
//
//	host := cfg.StringOr("server.host", "localhost")
func (c *Config) StringOr(key, defaultVal string) string {
	if c == nil {
		return defaultVal
	}
	val := c.Get(key)
	if val == nil {
		return defaultVal
	}
	return cast.ToString(val)
}

// IntOr returns the value associated with the given key as an int, or the default value if not found.
//
// Example:
//
//	port := cfg.IntOr("server.port", 8080)
func (c *Config) IntOr(key string, defaultVal int) int {
	if c == nil {
		return defaultVal
	}
	val := c.Get(key)
	if val == nil {
		return defaultVal
	}
	return cast.ToInt(val)
}

// Int64Or returns the value associated with the given key as an int64, or the default value if not found.
//
// Example:
//
//	maxSize := cfg.Int64Or("max_size", 1024)
func (c *Config) Int64Or(key string, defaultVal int64) int64 {
	if c == nil {
		return defaultVal
	}
	val := c.Get(key)
	if val == nil {
		return defaultVal
	}
	return cast.ToInt64(val)
}

// Float64Or returns the value associated with the given key as a float64, or the default value if not found.
//
// Example:
//
//	rate := cfg.Float64Or("rate", 0.5)
func (c *Config) Float64Or(key string, defaultVal float64) float64 {
	if c == nil {
		return defaultVal
	}
	val := c.Get(key)
	if val == nil {
		return defaultVal
	}
	return cast.ToFloat64(val)
}

// BoolOr returns the value associated with the given key as a boolean, or the default value if not found.
//
// Example:
//
//	debug := cfg.BoolOr("debug", false)
func (c *Config) BoolOr(key string, defaultVal bool) bool {
	if c == nil {
		return defaultVal
	}
	val := c.Get(key)
	if val == nil {
		return defaultVal
	}
	return cast.ToBool(val)
}

// DurationOr returns the value associated with the given key as a time.Duration, or the default value if not found.
//
// Example:
//
//	timeout := cfg.DurationOr("timeout", 30*time.Second)
func (c *Config) DurationOr(key string, defaultVal time.Duration) time.Duration {
	if c == nil {
		return defaultVal
	}
	val := c.Get(key)
	if val == nil {
		return defaultVal
	}
	return cast.ToDuration(val)
}

// TimeOr returns the value associated with the given key as a time.Time, or the default value if not found.
//
// Example:
//
//	startTime := cfg.TimeOr("start_time", time.Now())
func (c *Config) TimeOr(key string, defaultVal time.Time) time.Time {
	if c == nil {
		return defaultVal
	}
	val := c.Get(key)
	if val == nil {
		return defaultVal
	}
	return cast.ToTime(val)
}

// StringSliceOr returns the value associated with the given key as a slice of strings, or the default value if not found.
//
// Example:
//
//	tags := cfg.StringSliceOr("tags", []string{"default"})
func (c *Config) StringSliceOr(key string, defaultVal []string) []string {
	if c == nil {
		return defaultVal
	}
	val := c.Get(key)
	if val == nil {
		return defaultVal
	}
	return cast.ToStringSlice(val)
}

// IntSliceOr returns the value associated with the given key as a slice of integers, or the default value if not found.
//
// Example:
//
//	ports := cfg.IntSliceOr("ports", []int{8080, 8081})
func (c *Config) IntSliceOr(key string, defaultVal []int) []int {
	if c == nil {
		return defaultVal
	}
	val := c.Get(key)
	if val == nil {
		return defaultVal
	}
	return cast.ToIntSlice(val)
}

// StringMapOr returns the value associated with the given key as a map[string]any, or the default value if not found.
//
// Example:
//
//	metadata := cfg.StringMapOr("metadata", map[string]any{"version": "1.0"})
func (c *Config) StringMapOr(key string, defaultVal map[string]any) map[string]any {
	if c == nil {
		return defaultVal
	}
	val := c.Get(key)
	if val == nil {
		return defaultVal
	}
	return cast.ToStringMap(val)
}
