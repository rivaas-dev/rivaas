package internal

import (
	"fmt"
	"github.com/google/jsonapi"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/db"
	"gitlab.ci.fdmg.org/ci-api/cigourn"
	"gitlab.ci.fdmg.org/ci-api/cigourn/api"
	"gitlab.ci.fdmg.org/ci-api/cigourn/salesforce"
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/keycloak"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"math"
)

type PaginationParams struct {
	Size uint16
	Page uint32
}

// WriteJSONAPIResponse writes a json api response.
func WriteJSONAPIResponse(ctx *goskell.Context, body any, statusCode int, headers map[string]string) {
	for key, value := range headers {
		ctx.Header(key, value)
	}
	ctx.Header("Content-Type", jsonapi.MediaType)
	ctx.JSON(statusCode, body)
}

// GeneratePageLinks generates page links.
func GeneratePageLinks(paginationParams *PaginationParams, totalResult uint, urlPrefix string) *jsonapi.Links {
	// Generate links.
	var url = fmt.Sprintf("/v2%s?", urlPrefix)

	// Pages.
	links := jsonapi.Links{
		"self": fmt.Sprintf(
			"%s%s=%d&%s=%d",
			url,
			jsonapi.QueryParamPageSize,
			paginationParams.Size,
			jsonapi.QueryParamPageNumber,
			paginationParams.Page,
		),
	}
	totalPage := uint32(math.Ceil(float64(totalResult) / float64(paginationParams.Size)))

	// Next page.
	if totalPage > paginationParams.Page {
		links[jsonapi.KeyNextPage] = fmt.Sprintf(
			"%s%s=%d&%s=%d",
			url,
			jsonapi.QueryParamPageSize,
			paginationParams.Size,
			jsonapi.QueryParamPageNumber,
			paginationParams.Page+1,
		)
		links[jsonapi.KeyLastPage] = fmt.Sprintf(
			"%s%s=%d&%s=%d",
			url,
			jsonapi.QueryParamPageSize,
			paginationParams.Size,
			jsonapi.QueryParamPageNumber,
			totalPage,
		)
	}

	// Previous page.
	if paginationParams.Page > 1 && paginationParams.Page <= totalPage {
		links[jsonapi.KeyPreviousPage] = fmt.Sprintf(
			"%s%s=%d&%s=%d",
			url,
			jsonapi.QueryParamPageSize,
			paginationParams.Size,
			jsonapi.QueryParamPageNumber,
			paginationParams.Page-1,
		)
		links[jsonapi.KeyFirstPage] = fmt.Sprintf(
			"%s%s=%d&%s=1",
			url,
			jsonapi.QueryParamPageSize,
			paginationParams.Size,
			jsonapi.QueryParamPageNumber,
		)
	}

	return &links
}

// GetCustomerName compares the keycloak group id with the database key actor id.
func GetCustomerName(keycloakGroups []*keycloak.Group, key db.Key) string {
	// Set default as unknown
	customerName := "UNKNOWN"
	// Parse the URN
	parsedURN, err := cigourn.Parse(key.ActorID)
	if err != nil {
		return customerName
	}
	// Check if the parsed urn is of account type api key or salesforce
	_, TypeApi := parsedURN.(*api.Key)
	_, TypeSalesForce := parsedURN.(*salesforce.Account)
	// Validate
	if !TypeApi && !TypeSalesForce {
		return customerName
	}
	// Check in the keycloak groups for a matching customerId on api.key & salesforce.account
	for _, gr := range keycloakGroups {
		// For api account
		if TypeApi && *gr.ID == parsedURN.(*api.Key).CustomerID {
			return *gr.Name
		}
		// For salesforce account
		if TypeSalesForce && *gr.ID == parsedURN.(*salesforce.Account).AccountID {
			return *gr.Name
		}
	}

	return customerName
}
