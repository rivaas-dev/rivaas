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

//go:build !windows

package app

import (
	"os"
	"os/signal"
	"syscall"
)

// setupReloadSignal sets up SIGHUP signal handling for reload functionality.
// Returns a receive-only channel that receives SIGHUP signals and a cleanup function.
// The cleanup function should be called to stop signal notifications.
func setupReloadSignal() (<-chan os.Signal, func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGHUP)
	return ch, func() { signal.Stop(ch) }
}

// ignoreReloadSignal makes the process ignore SIGHUP so it is not terminated
// (e.g. by kill -HUP or terminal disconnect). Used when no OnReload hooks are registered.
func ignoreReloadSignal() {
	signal.Ignore(syscall.SIGHUP)
}
