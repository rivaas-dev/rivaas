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

package export

import (
	"testing"

	"rivaas.dev/openapi/model"
)

// createTestSpec creates a test spec for benchmarking.
func createTestSpec() *model.Spec {
	return &model.Spec{
		Info: model.Info{
			Title:       "Test API",
			Version:     "1.0.0",
			Description: "A test API for benchmarking",
		},
		Servers: []model.Server{
			{
				URL:         "https://api.example.com",
				Description: "Production server",
			},
		},
		Paths: map[string]*model.PathItem{
			"/users": {
				Get: &model.Operation{
					Summary:     "List users",
					Description: "Retrieve a list of users",
					OperationID: "listUsers",
					Responses: map[string]*model.Response{
						"200": {
							Description: "Success",
							Content: map[string]*model.MediaType{
								"application/json": {
									Schema: &model.Schema{
										Kind: model.KindArray,
										Items: &model.Schema{
											Kind: model.KindObject,
											Properties: map[string]*model.Schema{
												"id": {
													Kind: model.KindInteger,
												},
												"name": {
													Kind: model.KindString,
												},
											},
										},
									},
								},
							},
						},
					},
				},
				Post: &model.Operation{
					Summary:     "Create user",
					Description: "Create a new user",
					OperationID: "createUser",
					RequestBody: &model.RequestBody{
						Required: true,
						Content: map[string]*model.MediaType{
							"application/json": {
								Schema: &model.Schema{
									Kind: model.KindObject,
									Properties: map[string]*model.Schema{
										"name": {
											Kind:        model.KindString,
											Description: "User name",
										},
									},
									Required: []string{"name"},
								},
							},
						},
					},
					Responses: map[string]*model.Response{
						"201": {
							Description: "Created",
						},
					},
				},
			},
			"/users/{id}": {
				Get: &model.Operation{
					Summary:     "Get user",
					Description: "Retrieve a user by ID",
					OperationID: "getUser",
					Parameters: []model.Parameter{
						{
							Name:        "id",
							In:          "path",
							Required:    true,
							Description: "User ID",
							Schema: &model.Schema{
								Kind: model.KindInteger,
							},
						},
					},
					Responses: map[string]*model.Response{
						"200": {
							Description: "Success",
						},
					},
				},
			},
		},
		Components: &model.Components{
			Schemas: map[string]*model.Schema{
				"User": {
					Kind:        model.KindObject,
					Title:       "User",
					Description: "A user object",
					Properties: map[string]*model.Schema{
						"id": {
							Kind:        model.KindInteger,
							Description: "User ID",
						},
						"name": {
							Kind:        model.KindString,
							Description: "User name",
						},
						"email": {
							Kind:        model.KindString,
							Format:      "email",
							Description: "User email",
						},
					},
					Required: []string{"id", "name"},
				},
			},
		},
	}
}

func BenchmarkProject_30(b *testing.B) {
	spec := createTestSpec()
	cfg := Config{
		Version: V30,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := Project(spec, cfg, nil, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkProject_31(b *testing.B) {
	spec := createTestSpec()
	cfg := Config{
		Version: V31,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := Project(spec, cfg, nil, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkProject_30_Parallel(b *testing.B) {
	spec := createTestSpec()
	cfg := Config{
		Version: V30,
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _, err := Project(spec, cfg, nil, nil)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkProject_31_Parallel(b *testing.B) {
	spec := createTestSpec()
	cfg := Config{
		Version: V31,
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _, err := Project(spec, cfg, nil, nil)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkSchema30(b *testing.B) {
	schema := &model.Schema{
		Kind:        model.KindObject,
		Title:       "User",
		Description: "A user object",
		Properties: map[string]*model.Schema{
			"id": {
				Kind: model.KindInteger,
			},
			"name": {
				Kind: model.KindString,
			},
			"email": {
				Kind:   model.KindString,
				Format: "email",
			},
		},
		Required: []string{"id", "name"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var warns []Warning
		_ = schema30(schema, &warns, "#/test")
	}
}

func BenchmarkSchema31(b *testing.B) {
	schema := &model.Schema{
		Kind:        model.KindObject,
		Title:       "User",
		Description: "A user object",
		Properties: map[string]*model.Schema{
			"id": {
				Kind: model.KindInteger,
			},
			"name": {
				Kind: model.KindString,
			},
			"email": {
				Kind:   model.KindString,
				Format: "email",
			},
		},
		Required: []string{"id", "name"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var warns []Warning
		_ = schema31(schema, &warns, "#/test")
	}
}

func BenchmarkProject_30_WithExtensions(b *testing.B) {
	spec := createTestSpec()
	spec.Extensions = map[string]any{
		"x-api-version": "v1",
		"x-custom":      "value",
		"x-metadata": map[string]any{
			"author": "test",
			"tags":   []string{"api", "users"},
		},
	}
	cfg := Config{
		Version: V30,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := Project(spec, cfg, nil, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkProject_30_WithWarnings(b *testing.B) {
	spec := createTestSpec()
	spec.Info.Summary = "API summary" // 3.1-only
	spec.Webhooks = map[string]*model.PathItem{
		"userCreated": {},
	}
	cfg := Config{
		Version: V30,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := Project(spec, cfg, nil, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkProject_Allocations(b *testing.B) {
	spec := createTestSpec()
	cfg := Config{
		Version: V30,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := Project(spec, cfg, nil, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}
