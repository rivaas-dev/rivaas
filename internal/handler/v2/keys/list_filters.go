package keys

import "gitlab.ci.fdmg.org/ci-api/admin-api/internal/db"

const (
	actorIDFilter     = `actorID`
	descriptionFilter = `description`
	customerIDFilter  = `customerID`
	accountIDFilter   = `accountID`
)

// NewAdminSearchParameters construct search parameters from maps struct for admin users
func NewAdminSearchParameters(filter, match map[string]string) db.SearchParams {
	return db.SearchParams{
		FilterParams: db.FilterParams{
			ActorID: filter[actorIDFilter],
		},
		MatchParams: db.MatchParams{
			Description: match[descriptionFilter],
			CustomerID:  match[customerIDFilter],
			AccountID:   match[accountIDFilter],
		},
	}
}

// NewCustomerSearchParameters construct search parameters from maps struct for regular customers
func NewCustomerSearchParameters(_, match map[string]string, requestCustomerID string) db.SearchParams {
	return db.SearchParams{
		MatchParams: db.MatchParams{
			CustomerID:  requestCustomerID, // regular customers can only see keys that belong to the organization/customer id
			AccountID:   match[accountIDFilter],
			Description: match[descriptionFilter],
		},
	}
}
