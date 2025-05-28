package db

import "gitlab.ci.fdmg.org/ci-api/admin-api/internal/date"

// SearchParams represents the search parameters.
type SearchParams struct {
	FilterParams
	MatchParams
}

// FilterParams are used for exact matching search
type FilterParams struct {
	ActorID     string
	Environment string
	ExpiresAt   *date.Date
	Active      *bool
}

// MatchParams are used for fuzzy matching search
type MatchParams struct {
	Description string
	CustomerID  string
	AccountID   string
}
