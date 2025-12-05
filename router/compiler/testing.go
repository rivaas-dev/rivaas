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

package compiler

import "sync"

// testContextParamWriter is a test implementation of ContextParamWriter.
// It stores parameters in a map for easy assertion in tests.
type testContextParamWriter struct {
	mu     sync.Mutex
	params map[string]string
	count  int32
}

// SetParam stores a parameter by index and key.
func (m *testContextParamWriter) SetParam(index int, key, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.params == nil {
		m.params = make(map[string]string)
	}
	m.params[key] = value
}

// SetParamMap stores a parameter by key (used for >8 parameters).
func (m *testContextParamWriter) SetParamMap(key, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.params == nil {
		m.params = make(map[string]string)
	}
	m.params[key] = value
}

// SetParamCount sets the total parameter count.
func (m *testContextParamWriter) SetParamCount(count int32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.count = count
}

// GetParam returns a parameter value by key.
func (m *testContextParamWriter) GetParam(key string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.params == nil {
		return "", false
	}
	v, ok := m.params[key]
	return v, ok
}

// GetCount returns the parameter count.
func (m *testContextParamWriter) GetCount() int32 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.count
}

// Reset clears all stored parameters.
func (m *testContextParamWriter) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.params = nil
	m.count = 0
}
