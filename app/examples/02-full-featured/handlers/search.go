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

// Package handlers provides search-related HTTP handlers for the full-featured
// example application.
package handlers

import (
	"net/http"
	"time"

	"rivaas.dev/app"
)

// Search performs a search with advanced query parameter binding.
// It binds query parameters from the URL, applies default values for missing
// parameters, adds tracing attributes, and returns search results.
//
// Example:
//
//	GET /search?q=test&page=1&page_size=10&sort_by=name
//	Returns search results based on the provided query parameters.
func Search(c *app.Context) {
	var params SearchParams
	if err := c.Bind(&params); err != nil {
		HandleError(c, WrapError(ErrInvalidInput, "%v", err))
		return
	}

	// Apply defaults if not provided
	if params.Page == 0 {
		params.Page = 1
	}
	if params.PageSize == 0 {
		params.PageSize = 10
	}
	if params.SortBy == "" {
		params.SortBy = "name"
	}

	c.SetSpanAttribute("search.query", params.Query)
	c.SetSpanAttribute("search.page", params.Page)
	c.SetSpanAttribute("search.page_size", params.PageSize)
	c.SetSpanAttribute("search.sort_by", params.SortBy)
	if params.Active != nil {
		c.SetSpanAttribute("search.active", *params.Active)
	}
	c.AddSpanEvent("search_executed")

	// Simulate search
	time.Sleep(25 * time.Millisecond)

	c.JSON(http.StatusOK, map[string]any{
		"query":     params.Query,
		"page":      params.Page,
		"page_size": params.PageSize,
		"tags":      params.Tags,
		"active":    params.Active,
		"sort_by":   params.SortBy,
		"results":   []string{"result1", "result2", "result3"},
		"total":     3,
	})
}
