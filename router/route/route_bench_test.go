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

package route

import (
	"net/url"
	"testing"
)

// ParseReversePattern Benchmarks

func BenchmarkParseReversePattern_Simple(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		_ = ParseReversePattern("/users")
	}
}

func BenchmarkParseReversePattern_WithParams(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		_ = ParseReversePattern("/users/:id/posts/:postId")
	}
}

func BenchmarkParseReversePattern_Complex(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		_ = ParseReversePattern("/api/v1/users/:userId/organizations/:orgId/teams/:teamId/members/:memberId")
	}
}

// BuildURL Benchmarks

func BenchmarkBuildURL_Simple(b *testing.B) {
	pattern := ParseReversePattern("/users")
	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		// Note: Error intentionally ignored in benchmark - we're measuring performance, not correctness
		_, _ = pattern.BuildURL(nil, nil)
	}
}

func BenchmarkBuildURL_WithParams(b *testing.B) {
	pattern := ParseReversePattern("/users/:id/posts/:postId")
	params := map[string]string{
		"id":     "123",
		"postId": "456",
	}
	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		// Note: Error intentionally ignored in benchmark - we're measuring performance, not correctness
		_, _ = pattern.BuildURL(params, nil)
	}
}

func BenchmarkBuildURL_WithQuery(b *testing.B) {
	pattern := ParseReversePattern("/users/:id")
	params := map[string]string{"id": "123"}
	query := url.Values{}
	query.Set("page", "1")
	query.Set("limit", "10")
	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		// Note: Error intentionally ignored in benchmark - we're measuring performance, not correctness
		_, _ = pattern.BuildURL(params, query)
	}
}

func BenchmarkBuildURL_Complex(b *testing.B) {
	pattern := ParseReversePattern("/api/v1/users/:userId/organizations/:orgId/teams/:teamId/members/:memberId")
	params := map[string]string{
		"userId":   "user-123",
		"orgId":    "org-456",
		"teamId":   "team-789",
		"memberId": "member-012",
	}
	query := url.Values{}
	query.Set("page", "1")
	query.Set("limit", "10")
	query.Set("sort", "name")
	query.Set("order", "asc")
	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		// Note: Error intentionally ignored in benchmark - we're measuring performance, not correctness
		_, _ = pattern.BuildURL(params, query)
	}
}

// ParamConstraint.ToRegexConstraint Benchmarks

func BenchmarkParamConstraint_ToRegexConstraint_Int(b *testing.B) {
	pc := ParamConstraint{Kind: ConstraintInt}
	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_ = pc.ToRegexConstraint("id")
	}
}

func BenchmarkParamConstraint_ToRegexConstraint_UUID(b *testing.B) {
	pc := ParamConstraint{Kind: ConstraintUUID}
	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_ = pc.ToRegexConstraint("uuid")
	}
}

func BenchmarkParamConstraint_ToRegexConstraint_Enum(b *testing.B) {
	pc := ParamConstraint{
		Kind: ConstraintEnum,
		Enum: []string{"draft", "published", "archived", "deleted", "pending"},
	}
	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_ = pc.ToRegexConstraint("status")
	}
}

func BenchmarkParamConstraint_ToRegexConstraint_Regex(b *testing.B) {
	pc := ParamConstraint{
		Kind:    ConstraintRegex,
		Pattern: `[a-zA-Z][a-zA-Z0-9._-]*`,
	}
	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_ = pc.ToRegexConstraint("slug")
	}
}

// ParamConstraint.Compile Benchmarks

func BenchmarkParamConstraint_Compile(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		pc := ParamConstraint{
			Kind:    ConstraintRegex,
			Pattern: `[a-zA-Z][a-zA-Z0-9._-]*`,
		}
		pc.Compile()
	}
}

// ConstraintFromPattern Benchmarks

func BenchmarkConstraintFromPattern_Simple(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		_ = ConstraintFromPattern("id", `\d+`)
	}
}

func BenchmarkConstraintFromPattern_Complex(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		_ = ConstraintFromPattern("uuid", `[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}`)
	}
}

// BuildMountConfig Benchmarks

func BenchmarkBuildMountConfig_Empty(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		_ = BuildMountConfig()
	}
}

func BenchmarkBuildMountConfig_WithOptions(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		_ = BuildMountConfig(
			InheritMiddleware(),
			NamePrefix("api.v1."),
		)
	}
}

// Parallel Benchmarks

func BenchmarkParseReversePattern_Parallel(b *testing.B) {
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = ParseReversePattern("/users/:id/posts/:postId")
		}
	})
}

func BenchmarkBuildURL_Parallel(b *testing.B) {
	pattern := ParseReversePattern("/users/:id/posts/:postId")
	params := map[string]string{
		"id":     "123",
		"postId": "456",
	}
	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Note: Error intentionally ignored in benchmark - we're measuring performance, not correctness
			_, _ = pattern.BuildURL(params, nil)
		}
	})
}

func BenchmarkParamConstraint_ToRegexConstraint_Parallel(b *testing.B) {
	pc := ParamConstraint{Kind: ConstraintInt}
	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = pc.ToRegexConstraint("id")
		}
	})
}
