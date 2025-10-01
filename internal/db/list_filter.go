package db

import (
	"time"
)

// SearchParams represents the search parameters.
type SearchParams struct {
	FilterParams
	MatchParams
}

// FilterParams are used for exact matching search
type FilterParams struct {
	ActorID       string
	Environment   string
	ExpiresAt     *time.Time
	ExpiresBefore *time.Time
	ExpiresAfter  *time.Time
	Active        *bool
	Deleted       *bool
}

// MatchParams are used for fuzzy matching search
type MatchParams struct {
	Name        string
	Description string
	CustomerID  string
	AccountID   string
}

func Pointer[T any](value T) *T {
	return &value
}
