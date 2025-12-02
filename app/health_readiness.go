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
	"sync"
)

// Gate represents a component that reports its readiness status.
// Used for runtime registration of readiness checks that need to be
// dynamically added or removed during application lifecycle.
//
// For static readiness checks configured at startup, prefer using
// WithHealthEndpoints with WithReadinessCheck instead, as it provides
// better DX through the functional options pattern.
//
// Use Gate when you need:
//   - Dynamic registration/unregistration at runtime
//   - Component-owned readiness state (e.g., database connection pool)
//   - Integration with external libraries that manage their own state
type Gate interface {
	// Ready returns true if the component is ready to serve traffic.
	Ready() bool
	// Name returns the name of the gate for identification.
	Name() string
}

// ReadinessManager manages readiness gates for runtime health checks.
// ReadinessManager is safe for concurrent use by multiple goroutines.
//
// This complements the static [WithReadinessCheck] options by allowing
// dynamic registration and unregistration of readiness gates at runtime.
//
// Typical use cases:
//   - Database connection pools that manage their own health
//   - External service clients with retry/circuit breaker logic
//   - Components that need to temporarily mark themselves as not ready
type ReadinessManager struct {
	gates map[string]Gate
	mu    sync.RWMutex
}

// Register registers a readiness gate at runtime.
// If a gate with the same name already exists, it is replaced.
//
// Example:
//
//	type DatabaseGate struct {
//	    db *sql.DB
//	}
//	func (g *DatabaseGate) Ready() bool {
//	    return g.db.Ping() == nil
//	}
//	func (g *DatabaseGate) Name() string { return "database" }
//
//	app.Readiness().Register("db", &DatabaseGate{db: db})
func (rm *ReadinessManager) Register(name string, gate Gate) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if rm.gates == nil {
		rm.gates = make(map[string]Gate)
	}
	rm.gates[name] = gate
}

// Unregister removes a readiness gate by name.
// This is useful when a component is being shut down or
// is no longer relevant to the application's readiness.
//
// Example:
//
//	// During graceful shutdown of a specific component
//	app.Readiness().Unregister("database")
func (rm *ReadinessManager) Unregister(name string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	delete(rm.gates, name)
}

// Check checks if all registered gates are ready.
// Check returns true if all gates are ready, false otherwise.
// Check also returns a map of gate names to their readiness status.
//
// Example:
//
//	ready, status := app.Readiness().Check()
//	if !ready {
//	    for name, isReady := range status {
//	        if !isReady {
//	            log.Printf("Gate %s is not ready", name)
//	        }
//	    }
//	}
func (rm *ReadinessManager) Check() (bool, map[string]bool) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if len(rm.gates) == 0 {
		return true, nil // No gates = always ready
	}

	status := make(map[string]bool, len(rm.gates))
	allReady := true

	for name, gate := range rm.gates {
		ready := gate.Ready()
		status[name] = ready
		if !ready {
			allReady = false
		}
	}

	return allReady, status
}
