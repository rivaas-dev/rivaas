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

//go:build windows

package app

import "os"

// setupReloadSignal is a stub for Windows where SIGHUP is not available.
// Returns a nil channel (which blocks forever in select) and a no-op cleanup function.
// Users can still call Reload() programmatically on Windows.
func setupReloadSignal() (<-chan os.Signal, func()) {
	return nil, func() {}
}

// ignoreReloadSignal is a no-op on Windows where SIGHUP does not exist.
func ignoreReloadSignal() {}
