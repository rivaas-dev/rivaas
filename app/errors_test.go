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
