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

//go:build !integration

package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeGate implements Gate for tests with controllable readiness.
type fakeGate struct {
	ready bool
	name  string
}

func (g *fakeGate) Ready() bool  { return g.ready }
func (g *fakeGate) Name() string { return g.name }

func TestReadinessManager_Register(t *testing.T) {
	t.Parallel()

	app := MustNew(WithServiceName("test"), WithServiceVersion("1.0.0"))
	require.NotNil(t, app)
	rm := app.Readiness()

	gate := &fakeGate{ready: true, name: "db"}
	rm.Register("db", gate)

	ready, status := rm.Check()
	assert.True(t, ready)
	assert.NotNil(t, status)
	assert.True(t, status["db"])
}

func TestReadinessManager_Register_replacesExisting(t *testing.T) {
	t.Parallel()

	app := MustNew(WithServiceName("test"), WithServiceVersion("1.0.0"))
	require.NotNil(t, app)
	rm := app.Readiness()

	rm.Register("svc", &fakeGate{ready: false, name: "svc"})
	rm.Register("svc", &fakeGate{ready: true, name: "svc"})

	ready, status := rm.Check()
	assert.True(t, ready)
	assert.True(t, status["svc"])
}

func TestReadinessManager_Unregister(t *testing.T) {
	t.Parallel()

	app := MustNew(WithServiceName("test"), WithServiceVersion("1.0.0"))
	require.NotNil(t, app)
	rm := app.Readiness()

	rm.Register("a", &fakeGate{ready: true, name: "a"})
	rm.Unregister("a")

	ready, status := rm.Check()
	assert.True(t, ready)
	assert.Nil(t, status)
}

func TestReadinessManager_Unregister_idempotent(t *testing.T) {
	t.Parallel()

	app := MustNew(WithServiceName("test"), WithServiceVersion("1.0.0"))
	require.NotNil(t, app)
	rm := app.Readiness()

	rm.Unregister("nonexistent")
	rm.Unregister("nonexistent")

	ready, status := rm.Check()
	assert.True(t, ready)
	assert.Nil(t, status)
}

func TestReadinessManager_Check(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		register   func(*ReadinessManager)
		wantReady  bool
		wantStatus map[string]bool
	}{
		{
			name:       "no gates returns ready",
			register:   func(rm *ReadinessManager) {},
			wantReady:  true,
			wantStatus: nil,
		},
		{
			name: "all gates ready",
			register: func(rm *ReadinessManager) {
				rm.Register("a", &fakeGate{ready: true, name: "a"})
				rm.Register("b", &fakeGate{ready: true, name: "b"})
			},
			wantReady:  true,
			wantStatus: map[string]bool{"a": true, "b": true},
		},
		{
			name: "one gate not ready",
			register: func(rm *ReadinessManager) {
				rm.Register("ok", &fakeGate{ready: true, name: "ok"})
				rm.Register("fail", &fakeGate{ready: false, name: "fail"})
			},
			wantReady:  false,
			wantStatus: map[string]bool{"ok": true, "fail": false},
		},
		{
			name: "multiple gates mixed",
			register: func(rm *ReadinessManager) {
				rm.Register("r1", &fakeGate{ready: true, name: "r1"})
				rm.Register("r2", &fakeGate{ready: false, name: "r2"})
				rm.Register("r3", &fakeGate{ready: true, name: "r3"})
			},
			wantReady:  false,
			wantStatus: map[string]bool{"r1": true, "r2": false, "r3": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			app := MustNew(WithServiceName("test"), WithServiceVersion("1.0.0"))
			require.NotNil(t, app)
			rm := app.Readiness()
			tt.register(rm)

			ready, status := rm.Check()
			assert.Equal(t, tt.wantReady, ready)
			assert.Equal(t, tt.wantStatus, status)
		})
	}
}
