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

package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"time"

	"rivaas.dev/config"
	"rivaas.dev/config/codec"
)

// Config is the configuration for the application.
type Config struct {
	Foo     string        `config:"foo"`
	Timeout time.Duration `config:"timeout"`
	Debug   bool          `config:"debug"`
	Worker  Worker        `config:"worker"`
	Date    time.Time     `config:"date"`
	Roles   []string      `config:"roles"`
	Types   []string      `config:"types"`
	Types2  string        `config:"types"`
}

// Worker is the worker configuration.
type Worker struct {
	Timeout time.Duration `config:"timeout"`
	Address *url.URL      `config:"address"`
}

// main is the main function.
func main() {
	var cfg Config

	c := config.MustNew(
		config.WithFileSource("./config.yaml", codec.TypeYAML),
		config.WithBinding(&cfg),
	)

	err := c.Load(context.Background())
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	fmt.Printf("%+v\n", cfg)
}
