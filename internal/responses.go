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
	"net/url"
)

type PaginationParams struct {
	Size uint
	Page uint
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
func GeneratePageLinks(ctx *goskell.Context, paginationParams *PaginationParams, totalResult uint) (*jsonapi.Links, error) {
	// Get the previous path
	base, err := url.Parse(ctx.Request.URL.String())
	if err != nil {
		return nil, err
	}
	queries := base.Query()

	// Cleanup the 'page' parameters.
	queries.Del("page[size]")
	queries.Del("page[number]")

	// Generate base url.
	base.RawQuery = queries.Encode()
	baseurl, err := url.QueryUnescape(fmt.Sprintf("%s?%s", base.Path, base.RawQuery))
	if err != nil {
		return nil, err
	}

	fmtURL := "%s&%s=%d&%s=%d"
	if base.RawQuery == "" { // if there are no query parameters, skip the '&'
		fmtURL = "%s%s=%d&%s=%d"
	}
	// Pages.
	links := jsonapi.Links{
		"self": fmt.Sprintf(
			fmtURL,
			baseurl,
			jsonapi.QueryParamPageSize,
			paginationParams.Size,
			jsonapi.QueryParamPageNumber,
			paginationParams.Page,
		),
	}
	totalPage := uint(math.Ceil(float64(totalResult) / float64(paginationParams.Size)))

	// Next page.
	if totalPage > paginationParams.Page {
		links[jsonapi.KeyNextPage] = fmt.Sprintf(
			fmtURL,
			baseurl,
			jsonapi.QueryParamPageSize,
			paginationParams.Size,
			jsonapi.QueryParamPageNumber,
			paginationParams.Page+1,
		)
		links[jsonapi.KeyLastPage] = fmt.Sprintf(
			fmtURL,
			baseurl,
			jsonapi.QueryParamPageSize,
			paginationParams.Size,
			jsonapi.QueryParamPageNumber,
			totalPage,
		)
	}

	// Previous page.
	if paginationParams.Page > 1 && paginationParams.Page <= totalPage {
		links[jsonapi.KeyPreviousPage] = fmt.Sprintf(
			fmtURL,
			baseurl,
			jsonapi.QueryParamPageSize,
			paginationParams.Size,
			jsonapi.QueryParamPageNumber,
			paginationParams.Page-1,
		)
		links[jsonapi.KeyFirstPage] = fmt.Sprintf(
			fmtURL,
			baseurl,
			jsonapi.QueryParamPageSize,
			paginationParams.Size,
			jsonapi.QueryParamPageNumber,
			1,
		)
	}

	return &links, nil
}

// GetCustomerName compares the keycloak group id with the database key actor id.
func GetCustomerName(keycloakGroups []*keycloak.Group, key db.Key) string {
	return GetCustomerNameByActorID(keycloakGroups, key.ActorID)
}

// GetCustomerNameByActorID compares the keycloak group id with the database key actor id.
func GetCustomerNameByActorID(keycloakGroups []*keycloak.Group, actorID string) string {
	// Set default as unknown
	customerName := "UNKNOWN"
	// Parse the URN
	parsedURN, err := cigourn.Parse(actorID)
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
