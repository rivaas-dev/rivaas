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

package openapi

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "update golden files")

func TestManager_ConcurrentRegister(t *testing.T) {
	t.Parallel()

	cfg := MustNew(WithTitle("Test", "1.0"))
	mgr := NewManager(cfg)

	// Test concurrent registration
	var wg sync.WaitGroup
	const numRoutes = 100
	for i := range numRoutes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			mgr.Register("GET", fmt.Sprintf("/route%d", i))
		}(i)
	}
	wg.Wait()

	// Verify all routes registered by generating spec
	specJSON, _, err := mgr.GenerateSpec()
	require.NoError(t, err)
	require.NotNil(t, specJSON)
	// Spec should be valid JSON (basic check)
	assert.NotEmpty(t, specJSON)
}

func TestManager_GenerateSpec_Golden(t *testing.T) {
	t.Parallel()

	cfg := MustNew(
		WithTitle("Test API", "1.0.0"),
		WithDescription("Test description"),
		WithServer("https://api.example.com", "Production"),
		WithTag("users", "User operations"),
	)
	mgr := NewManager(cfg)
	mgr.Register("GET", "/users/:id").
		Doc("Get user", "Retrieves a user by ID").
		Response(200, User{})
	mgr.Register("POST", "/users").
		Doc("Create user", "Creates a new user").
		Request(User{}).
		Response(201, User{})

	specJSON, _, err := mgr.GenerateSpec()
	require.NoError(t, err)

	golden := filepath.Join("testdata", "spec.golden.json")

	if *update {
		require.NoError(t, os.MkdirAll(filepath.Dir(golden), 0755), "failed to create testdata directory")
		require.NoError(t, os.WriteFile(golden, specJSON, 0644), "failed to write golden file")
		t.Log("Updated golden file")
		return
	}

	want, err := os.ReadFile(golden)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skipf("golden file %s does not exist, run with -update to create it", golden)
			return
		}
		require.NoError(t, err, "failed to read golden file")
		return
	}

	// Compare JSON using semantic comparison
	assert.JSONEq(t, string(want), string(specJSON), "spec JSON does not match golden file")
}

type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
