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

package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigError_Error_WithoutHint(t *testing.T) {
	t.Parallel()
	e := &ConfigError{
		Field:   "serviceName",
		Message: "cannot be empty",
	}
	got := e.Error()
	assert.Equal(t, "configuration error in serviceName: cannot be empty", got)
	assert.NotContains(t, got, "WithServiceName")
}

func TestConfigError_Error_WithHint(t *testing.T) {
	t.Parallel()
	e := &ConfigError{
		Field:   "serviceName",
		Message: "cannot be empty",
		Hint:    "use app.WithServiceName(\"...\") or set RIVAAS_SERVICE_NAME",
	}
	got := e.Error()
	assert.Contains(t, got, "configuration error in serviceName: cannot be empty")
	assert.Contains(t, got, "use app.WithServiceName")
	assert.Contains(t, got, "RIVAAS_SERVICE_NAME")
}

func TestConfigErrors_Error_Empty(t *testing.T) {
	t.Parallel()
	var ce ConfigErrors
	got := ce.Error()
	assert.Equal(t, "config errors: (no errors)", got)
}

func TestConfigErrors_Error_One(t *testing.T) {
	t.Parallel()
	ce := &ConfigErrors{}
	ce.Add(&ConfigError{Field: "port", Message: "must be positive"})
	got := ce.Error()
	assert.Equal(t, "configuration error in port: must be positive", got)
}

func TestConfigErrors_Error_Multiple(t *testing.T) {
	t.Parallel()
	ce := &ConfigErrors{}
	ce.Add(&ConfigError{Field: "a", Message: "first"})
	ce.Add(&ConfigError{Field: "b", Message: "second"})
	got := ce.Error()
	assert.Contains(t, got, "config errors (2):")
	assert.Contains(t, got, "1. configuration error in a: first")
	assert.Contains(t, got, "2. configuration error in b: second")
}

func TestConfigErrors_HasErrors(t *testing.T) {
	t.Parallel()
	var ce ConfigErrors
	assert.False(t, ce.HasErrors())
	ce.Add(&ConfigError{Field: "x", Message: "err"})
	assert.True(t, ce.HasErrors())
}

func TestConfigErrors_ToError(t *testing.T) {
	t.Parallel()
	var ce ConfigErrors
	assert.Nil(t, ce.ToError())
	ce.Add(&ConfigError{Field: "x", Message: "err"})
	err := ce.ToError()
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "configuration error in x")
}
