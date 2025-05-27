package db

// SearchParams represents the search parameters.
type SearchParams struct {
	FilterParams
	MatchParams
}

// FilterParams are used for exact matching search
type FilterParams struct {
	ActorID string
}

// MatchParams are used for fuzzy matching search
type MatchParams struct {
	Description string
	CustomerID  string
	AccountID   string
}
