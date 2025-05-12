package db

const (
	actorIDFilter     = `actorID`
	descriptionFilter = `description`
	customerIDFilter  = `customerID`
	accountIDFilter   = `accountID`
)

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

// NewSearchParameters construct search parameters from maps struct
func NewSearchParameters(filter, match map[string]string) SearchParams {
	return SearchParams{
		FilterParams: FilterParams{
			ActorID: filter[actorIDFilter],
		},
		MatchParams: MatchParams{
			Description: match[descriptionFilter],
			CustomerID:  match[customerIDFilter],
			AccountID:   match[accountIDFilter],
		},
	}
}
