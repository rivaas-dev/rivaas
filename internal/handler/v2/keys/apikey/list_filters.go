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
	nameFilter        = `name`
	descriptionFilter = `description`
	environmentFilter = `environment`
	expiresAtFilter   = `expiresAt`
	activeFilter      = `active`
	isDeletedFilter   = `isDeleted`
)

type FilterParam struct {
	Filter map[string]string
	Match  map[string]string
}

// NewAdministratorSearchParameters construct search parameters from maps struct for administrator users
func NewAdministratorSearchParameters(filter, match map[string]string) (db.SearchParams, error) {
	expiresAtParam, err := paramToDate(filter[expiresAtFilter])
	if err != nil {
		return db.SearchParams{}, fmt.Errorf("invalid filter `%s`: %w", expiresAtFilter, err)
	}

	activeParam, err := paramToBool(filter[activeFilter])
	if err != nil {
		return db.SearchParams{}, fmt.Errorf("invalid filter `%s`: %w", activeFilter, err)
	}

	deletedParam, err := paramToBool(filter[isDeletedFilter])
	if err != nil {
		return db.SearchParams{}, fmt.Errorf("invalid filter `%s`: %w", isDeletedFilter, err)
	}

	// set the default value to true so the list isn't bombarded with unnecessary key
	if deletedParam == nil {
		deleted := false
		deletedParam = &deleted
	}

	return db.SearchParams{
		FilterParams: db.FilterParams{
			ActorID:     filter[actorIDFilter],
			Environment: filter[environmentFilter],
			ExpiresAt:   expiresAtParam,
			Active:      activeParam,
			Deleted:     deletedParam,
		},
		MatchParams: db.MatchParams{
			Name:        match[nameFilter],
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

	// actorID filter is not allowed for customers
	// the can only see their own keys
	if filter, ok := filter[actorIDFilter]; ok && filter != "" {
		return db.SearchParams{}, fmt.Errorf("invalid filter `%s`", actorIDFilter)
	}

	expiresAtParam, err := paramToDate(filter[expiresAtFilter])
	if err != nil {
		return db.SearchParams{}, fmt.Errorf("invalid filter `%s`: %w", expiresAtFilter, err)
	}

	activeParam, err := paramToBool(filter[activeFilter])
	if err != nil {
		return db.SearchParams{}, fmt.Errorf("invalid filter `%s`: %w", activeFilter, err)
	}

	deletedParam, err := paramToBool(filter[isDeletedFilter])
	if err != nil {
		return db.SearchParams{}, fmt.Errorf("invalid filter `%s`: %w", isDeletedFilter, err)
	}

	// set the default value to true so the list isn't bombarded with unnecessary keys
	if deletedParam == nil {
		deleted := false
		deletedParam = &deleted
	}

	return db.SearchParams{
		FilterParams: db.FilterParams{
			Environment: filter[environmentFilter],
			ExpiresAt:   expiresAtParam,
			Active:      activeParam,
			Deleted:     deletedParam,
		},
		MatchParams: db.MatchParams{
			Name:        match[nameFilter],
			Description: match[descriptionFilter],
			CustomerID:  requestCustomerID, // regular customers can only see keys that belong to their organization/customer id
			AccountID:   match[accountIDFilter],
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
