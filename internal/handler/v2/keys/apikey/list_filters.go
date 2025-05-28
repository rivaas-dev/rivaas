package apikey

import (
	"errors"
	"fmt"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/date"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/db"
	"strconv"
)

const (
	actorIDFilter     = `actorID`
	customerIDFilter  = `customerID`
	accountIDFilter   = `accountID`
	descriptionFilter = `description`
	environmentFilter = `environment`
	expiresAtFilter   = `expiresAt`
	activeFilter      = `active`
)

type FilterParam struct {
	Filter map[string]string
	Match  map[string]string
}

// NewAdminSearchParameters construct search parameters from maps struct for admin users
func NewAdminSearchParameters(filter, match map[string]string) (db.SearchParams, error) {
	expiresAtParam, err := paramToDate(filter[expiresAtFilter])
	if err != nil {
		return db.SearchParams{}, fmt.Errorf("invalid filter `%s`: %w", expiresAtFilter, err)
	}

	activeParam, err := paramToBool(filter[activeFilter])
	if err != nil {
		return db.SearchParams{}, fmt.Errorf("invalid filter `%s`: %w", activeFilter, err)
	}

	return db.SearchParams{
		FilterParams: db.FilterParams{
			ActorID:     filter[actorIDFilter],
			Environment: filter[environmentFilter],
			ExpiresAt:   expiresAtParam,
			Active:      activeParam,
		},
		MatchParams: db.MatchParams{
			Description: match[descriptionFilter],
			CustomerID:  match[customerIDFilter],
			AccountID:   match[accountIDFilter],
		},
	}, nil
}

// NewCustomerSearchParameters construct search parameters from maps struct for regular customers
func NewCustomerSearchParameters(filter, match map[string]string, requestCustomerID string) (db.SearchParams, error) {
	// customerID filter is not allowed for customers
	// the can only see their own keys
	if filter, ok := match[customerIDFilter]; ok && filter != "" {
		return db.SearchParams{}, fmt.Errorf("invalid filter `%s`", customerIDFilter)
	}

	expiresAtParam, err := paramToDate(filter[expiresAtFilter])
	if err != nil {
		return db.SearchParams{}, fmt.Errorf("invalid filter `%s`: %w", expiresAtFilter, err)
	}

	activeParam, err := paramToBool(filter[activeFilter])
	if err != nil {
		return db.SearchParams{}, fmt.Errorf("invalid filter `%s`: %w", activeFilter, err)
	}

	return db.SearchParams{
		FilterParams: db.FilterParams{
			Environment: filter[environmentFilter],
			ExpiresAt:   expiresAtParam,
			Active:      activeParam,
		},
		MatchParams: db.MatchParams{
			CustomerID:  requestCustomerID, // regular customers can only see keys that belong to the organization/customer id
			AccountID:   match[accountIDFilter],
			Description: match[descriptionFilter],
		},
	}, nil
}

func paramToBool(param string) (*bool, error) {
	if param == "" {
		return nil, nil
	}

	out, err := strconv.ParseBool(param)
	if err != nil {
		return &out, errors.New("invalid value for boolean parameter")
	}

	return &out, nil
}

func paramToDate(param string) (*date.Date, error) {
	if param == "" {
		return nil, nil
	}

	result, err := date.Parse(param)
	if err != nil {
		return result, err
	}

	return result, nil
}
