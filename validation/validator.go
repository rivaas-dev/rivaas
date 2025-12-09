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

package validation

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-playground/validator/v10"
)

// Validator provides struct validation with configurable options.
//
// Use [New] or [MustNew] to create a configured Validator, or use package-level
// functions ([Validate], [ValidatePartial]) for zero-configuration validation.
//
// Validator supports three validation strategies (see [Strategy]):
//   - Struct tags via go-playground/validator ([StrategyTags])
//   - JSON Schema validation ([StrategyJSONSchema])
//   - Custom interface methods ([StrategyInterface])
//
// Validator is safe for concurrent use by multiple goroutines.
//
// Example:
//
//	validator := validation.MustNew(
//	    validation.WithRedactor(sensitiveRedactor),
//	    validation.WithMaxErrors(10),
//	)
//
//	err := validator.Validate(ctx, &user)
type Validator struct {
	cfg *config

	// Tag validator (go-playground/validator)
	tagValidator     *validator.Validate
	tagValidatorOnce sync.Once
	tagValidatorErr  error // stores init error for deferred checking

	// Schema cache for JSON Schema validation
	schemaCache   map[string]*schemaCacheEntry
	schemaCacheMu sync.RWMutex

	// Path cache: Type -> namespace -> JSON path
	pathCache sync.Map // map[reflect.Type]*sync.Map[string]string

	// Field map cache: Type -> JSON field name -> field index
	fieldMapCache sync.Map // map[reflect.Type]map[string]int

	// Interface type caches
	validatorTypeCache            sync.Map // map[reflect.Type]bool
	validatorWithContextTypeCache sync.Map // map[reflect.Type]bool
}

// New creates a [Validator] with the given options.
// New returns an error if configuration is invalid (e.g., negative maxErrors).
//
// See [Option] for available configuration options.
//
// Example:
//
//	validator, err := validation.New(
//	    validation.WithMaxErrors(10),
//	    validation.WithRedactor(sensitiveRedactor),
//	)
//	if err != nil {
//	    return fmt.Errorf("failed to create validator: %w", err)
//	}
func New(opts ...Option) (*Validator, error) {
	cfg := newConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	v := &Validator{
		cfg:         cfg,
		schemaCache: make(map[string]*schemaCacheEntry),
	}

	if err := v.initTagValidator(); err != nil {
		return nil, fmt.Errorf("initialize tag validator: %w", err)
	}

	return v, nil
}

// MustNew creates a [Validator] with the given options.
// Panics if configuration is invalid.
//
// Use in main() or init() where panic on startup is acceptable.
//
// Example:
//
//	validator := validation.MustNew(
//	    validation.WithRedactor(sensitiveRedactor),
//	    validation.WithMaxErrors(10),
//	)
func MustNew(opts ...Option) *Validator {
	v, err := New(opts...)
	if err != nil {
		panic(fmt.Sprintf("validation.MustNew: %v", err))
	}

	return v
}

// initTagValidator initializes the go-playground/validator instance for [StrategyTags].
// This method is safe for concurrent use.
func (v *Validator) initTagValidator() error {
	v.tagValidatorOnce.Do(func() {
		v.tagValidator = validator.New(validator.WithRequiredStructEnabled())

		// Use json tags as field names for better error messages
		v.tagValidator.RegisterTagNameFunc(func(fld reflect.StructField) string {
			name := fld.Tag.Get("json")
			if name == "-" {
				return ""
			}
			if idx := strings.Index(name, ","); idx != -1 {
				name = name[:idx]
			}
			if name == "" {
				return fld.Name
			}

			return name
		})

		if err := v.registerBuiltinValidators(); err != nil {
			v.tagValidatorErr = fmt.Errorf("register built-in validators: %w", err)
			return
		}

		for _, ct := range v.cfg.customTags {
			if err := v.tagValidator.RegisterValidation(ct.name, ct.fn); err != nil {
				v.tagValidatorErr = fmt.Errorf("register custom tag %q: %w", ct.name, err)
				return
			}
		}
	})

	return v.tagValidatorErr
}

// Built-in regex patterns for custom validators (username, slug).
var (
	reUsername = regexp.MustCompile(`^[a-zA-Z0-9_]{3,20}$`)
	reSlug     = regexp.MustCompile(`^[a-z0-9-]+$`)
)

// registerBuiltinValidators registers the built-in custom validators: username, slug, strong_password.
func (v *Validator) registerBuiltinValidators() error {
	if err := v.tagValidator.RegisterValidation("username", func(fl validator.FieldLevel) bool {
		return reUsername.MatchString(fl.Field().String())
	}); err != nil {
		return fmt.Errorf("failed to register username validator: %w", err)
	}

	if err := v.tagValidator.RegisterValidation("slug", func(fl validator.FieldLevel) bool {
		return reSlug.MatchString(fl.Field().String())
	}); err != nil {
		return fmt.Errorf("failed to register slug validator: %w", err)
	}

	if err := v.tagValidator.RegisterValidation("strong_password", func(fl validator.FieldLevel) bool {
		return len(fl.Field().String()) >= 8
	}); err != nil {
		return fmt.Errorf("failed to register strong_password validator: %w", err)
	}

	return nil
}

// typeImplementsValidator checks if a type implements [ValidatorInterface].
func (v *Validator) typeImplementsValidator(t reflect.Type) bool {
	if cached, ok := v.validatorTypeCache.Load(t); ok {
		if result, resultOk := cached.(bool); resultOk {
			return result
		}
	}

	implements := t.Implements(reflect.TypeFor[ValidatorInterface]())

	actual, loaded := v.validatorTypeCache.LoadOrStore(t, implements)
	if loaded {
		if result, ok := actual.(bool); ok {
			return result
		}
	}

	return implements
}

// typeImplementsValidatorWithContext checks if a type implements [ValidatorWithContext].
func (v *Validator) typeImplementsValidatorWithContext(t reflect.Type) bool {
	if cached, ok := v.validatorWithContextTypeCache.Load(t); ok {
		if result, resultOk := cached.(bool); resultOk {
			return result
		}
	}

	implements := t.Implements(reflect.TypeFor[ValidatorWithContext]())

	actual, loaded := v.validatorWithContextTypeCache.LoadOrStore(t, implements)
	if loaded {
		if result, ok := actual.(bool); ok {
			return result
		}
	}

	return implements
}

// getFieldMap returns a map of JSON field names to field indices for a struct type.
func (v *Validator) getFieldMap(structType reflect.Type) map[string]int {
	if cached, ok := v.fieldMapCache.Load(structType); ok {
		if fieldMap, fieldMapOk := cached.(map[string]int); fieldMapOk {
			return fieldMap
		}
	}

	fieldMap := buildFieldMap(structType)

	actual, loaded := v.fieldMapCache.LoadOrStore(structType, fieldMap)
	if loaded {
		if result, ok := actual.(map[string]int); ok {
			return result
		}
	}

	return fieldMap
}

// getCachedJSONPath gets or computes JSON path from validator namespace.
func (v *Validator) getCachedJSONPath(ns string, structType reflect.Type) string {
	cacheVal, ok := v.pathCache.Load(structType)
	if !ok {
		newCache := &sync.Map{}
		actual, loaded := v.pathCache.LoadOrStore(structType, newCache)
		if loaded {
			cacheVal = actual
		} else {
			cacheVal = newCache
		}
	}

	typeCache, typeOk := cacheVal.(*sync.Map)
	if !typeOk {
		newCache := &sync.Map{}
		actual, loaded := v.pathCache.LoadOrStore(structType, newCache)
		if loaded {
			if tc, tcOk := actual.(*sync.Map); tcOk {
				typeCache = tc
			} else {
				typeCache = newCache
			}
		} else {
			typeCache = newCache
		}
	}

	if cached, cachedOk := typeCache.Load(ns); cachedOk {
		if result, resultOk := cached.(string); resultOk {
			return result
		}
	}

	jsonPath := namespaceToJSONPath(ns, structType)

	actual, loaded := typeCache.LoadOrStore(ns, jsonPath)
	if loaded {
		if result, resultOk := actual.(string); resultOk {
			return result
		}
	}

	return jsonPath
}

// getOrCompileSchema gets a JSON Schema from cache or compiles a new one for [StrategyJSONSchema].
func (v *Validator) getOrCompileSchema(id, schemaJSON string) (*jsonschemaSchema, error) {
	now := time.Now()

	if id != "" {
		v.schemaCacheMu.RLock()
		if entry, ok := v.schemaCache[id]; ok {
			schema := entry.schema
			v.schemaCacheMu.RUnlock()
			entry.lastAccess.Store(now.UnixNano())

			return schema, nil
		}
		v.schemaCacheMu.RUnlock()
	}

	schema, err := compileSchema(id, schemaJSON)
	if err != nil {
		return nil, err
	}

	if id != "" {
		v.schemaCacheMu.Lock()
		defer v.schemaCacheMu.Unlock()

		maxCache := v.cfg.maxCachedSchemas
		if maxCache == 0 {
			maxCache = defaultMaxCachedSchemas
		}

		if len(v.schemaCache) >= maxCache {
			var oldestID string
			var oldestNano int64
			found := false

			for cacheID, entry := range v.schemaCache {
				entryNano := entry.lastAccess.Load()
				if !found || entryNano < oldestNano {
					oldestID = cacheID
					oldestNano = entryNano
					found = true
				}
			}

			if found {
				delete(v.schemaCache, oldestID)
			}
		}

		entry := &schemaCacheEntry{
			schema: schema,
		}
		entry.lastAccess.Store(now.UnixNano())
		v.schemaCache[id] = entry
	}

	return schema, nil
}

// schemaCacheEntry holds a cached JSON Schema and its last access time for LRU eviction.
type schemaCacheEntry struct {
	schema     *jsonschemaSchema
	lastAccess atomic.Int64 // Unix nanoseconds for thread-safe access
}
