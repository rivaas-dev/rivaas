// Package keys defines all methods of the API key.
package keys

import (
	"errors"
	"github.com/companyinfo/jsonapi"
	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/date"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/db"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/handler/v2/keys/apikey"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/handler/v2/pagination"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/headers"
	"gitlab.ci.fdmg.org/datacluster/golibs/goot"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"math"
	"net/http"
)

// ListInput represents the request body of the LIST endpoint.
type ListInput struct {
	headers.Authorization

	Filter           map[string]string
	Match            map[string]string
	PaginationParams internal.PaginationParams
}

// LIST handles GET requests on the endpoint to get list of keys.
func (h *Handler) LIST(ctx *goskell.Context) {
	// Parse request body.
	request, err := bindRequest(ctx, h.defaultPageSize, h.maxPageSize)
	if err != nil {
		goskell.JsonAPIError(ctx, "input body validation", err, http.StatusBadRequest)
		return
	}

	// Fetch keys from the database.
	_, span := goot.Span(ctx.Request.Context(), "get_from_database")
	keys, totalResults, err := h.getAPIKeys(request)
	if err != nil {
		goot.EndSpanWithError(span, err, "failed to call database")
		log.Err(err).Msg("error while communicating with DB")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}
	goot.EndSpan(span)

	if !h.isAuthorized(ctx, nil) {
		// The appropriate response is already handled in "isAuthorized()"
		return
	}

	// Prepare the response.
	jsonAPIResponse, err := jsonapi.Marshal(h.convertListDBResultToJSON(ctx, keys))
	if err != nil {
		log.Err(err).Msg("failed to marshal JSON API response")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}

	//Adds the pagination links to the end of the response
	if ja, ok := jsonAPIResponse.(*jsonapi.ManyPayload); ok {
		links, err := internal.GeneratePageLinks(ctx,
			&internal.PaginationParams{
				Size: request.PaginationParams.Size,
				Page: request.PaginationParams.Page,
			}, uint(totalResults))
		if err != nil {
			log.Err(err).Msg("failed to generate pagination links")
			goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
			return
		}

		ja.Links = (*jsonapi.Links)(links)
		ja.Meta = &jsonapi.Meta{
			"totalResults": totalResults,
			"totalPages":   uint32(math.Ceil(float64(totalResults) / float64(request.PaginationParams.Size))),
		}
	}

	internal.WriteJSONAPIResponse(ctx, jsonAPIResponse, http.StatusOK, nil)
}

func (h *Handler) getAPIKeys(request *ListInput) (keys []*db.Key, totalResults int64, err error) {
	var searchParams db.SearchParams
	if headers.IsAdministrator(request.Authorization.Roles) {
		searchParams, err = apikey.NewAdministratorSearchParameters(request.Filter, request.Match)
	} else {
		searchParams, err = apikey.NewCustomerSearchParameters(request.Filter, request.Match, request.CustomerUser.CustomerID)
	}
	if err != nil {
		return nil, 0, err
	}

	return h.keysRepository.GetKeysPaginated(searchParams, request.PaginationParams.Size, request.PaginationParams.Page)
}

func (h *Handler) convertListDBResultToJSON(ctx *goskell.Context, keys []*db.Key) []*apikey.ListOutput {
	response := make([]*apikey.ListOutput, 0)
	for _, key := range keys {
		rl := apikey.RateLimit{
			Rate: 0,
			Per:  0,
		}

		if key.Metadata != nil {
			if data, ok := key.Metadata["rate_limit"]; ok {
				castedData := data.(map[string]any)
				rl.Rate = castedData["Rate"].(float64)
				rl.Per = castedData["Per"].(float64)
			}
		}

		// call keycloak to get the customer name, set it as unknown to avoid panic when not found
		keycloakGroups, err := h.keycloakClient.GetGroups(ctx, nil, h.keycloakConfig.BrifRepresentation)
		if err != nil {
			log.Err(err).Msg("cant find client in keycloak")
		}
		// Define the response
		response = append(response, &apikey.ListOutput{
			ID:           key.ID,
			Name:         key.Name,
			CustomerName: internal.GetCustomerName(keycloakGroups, *key),
			CreatorID:    key.CreatorID,
			Hash:         key.Hash,
			ActorID:      key.ActorID,
			Description:  key.Description,
			CreatedAt:    date.FormatTime(key.CreatedAt),
			ExpiresAt:    date.FormatTimeToPtr(key.ExpiresAt),
			DeletedAt:    date.FormatTimeToPtr(key.DeletedAt),
			Contact:      apikey.DBToContact(key.Contact),
			Active:       key.Active,
			RateLimit:    rl,
			Environment:  key.Environment,
			Labels:       key.Labels,
		})
	}
	return response
}

// bindRequest bind the request to the Request struct.
func bindRequest(ctx *goskell.Context, defaultPageSize, maxPageSize uint) (*ListInput, error) {
	var request ListInput

	err := ctx.ShouldBind(&request)
	if err != nil {
		return nil, err
	}

	request.Authorization, err = headers.GetAuthorization(ctx)
	if err != nil {
		return nil, err
	}

	request.PaginationParams, err = pagination.GetPagination(ctx, defaultPageSize, maxPageSize)
	if err != nil {
		log.Error().Err(err).Str("customerID", request.CustomerID).Msg("invalid pagination parameters")
		return nil, errors.New("invalid pagination parameters")
	}

	// Filtering
	request.Filter = ctx.QueryMap("filter")
	request.Match = ctx.QueryMap("match")
	if err := apikey.ValidateFilters(apikey.FilterParam{Filter: request.Filter, Match: request.Match}); err != nil {
		return nil, err
	}

	return &request, nil
}
