package customers

import (
	"github.com/companyinfo/jsonapi"
	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/handler/v2/customers/filters"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/handler/v2/pagination"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/keycloak"
	"gitlab.ci.fdmg.org/datacluster/golibs/goot"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"math"
	"net/http"
)

// ListInput represents the request body of the LIST endpoint.
type ListInput struct {
	Match map[string]string
	Query struct {
		PaginationParams internal.PaginationParams
	}
}

// LIST handles GET list of customer
func (h *Handler) LIST(ctx *goskell.Context) {
	// Parse request body.
	request, err := bindRequest(ctx, h.defaultPageSize, h.maxPageSize)
	if err != nil {
		goskell.JsonAPIError(ctx, "input body validation", err, http.StatusBadRequest)
		return
	}

	searchParams, err := filters.NewSearchParameters(filters.FilterParam{Match: request.Match})
	if err != nil {
		goskell.JsonAPIError(ctx, "invalid query parameters", err, http.StatusBadRequest)
		return
	}

	first := int((request.Query.PaginationParams.Page - 1) * request.Query.PaginationParams.Size)
	max := int(request.Query.PaginationParams.Size)

	// Fetch total group count
	_, span := goot.Span(ctx.Request.Context(), "get_group_count_from_keycloak")
	t := true
	totalResults, err := h.keycloakClient.GetGroupsCount(ctx, searchParams.Name, &t)
	if err != nil {
		goot.EndSpanWithError(span, err, "failed to call keycloak")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}
	goot.EndSpan(span)

	// Fetch the requested page of the groups
	_, span = goot.Span(ctx.Request.Context(), "get_from_keycloak")
	groups, err := h.keycloakClient.GetGroupsPaginated(ctx, searchParams.Name, h.keycloakConfig.BrifRepresentation, first, max)
	if err != nil {
		goot.EndSpanWithError(span, err, "failed to call keycloak")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}
	goot.EndSpan(span)

	// Parse the group data
	parsedKeyCloakGroups := keycloak.ParseCustomerGroups(groups)

	// Build response from groups
	jsonAPIResponse, err := jsonapi.Marshal(parsedKeyCloakGroups)
	if err != nil {
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}

	//Adds the pagination links to the end of the response
	if ja, ok := jsonAPIResponse.(*jsonapi.ManyPayload); ok {
		links, err := internal.GeneratePageLinks(ctx,
			&internal.PaginationParams{
				Size: request.Query.PaginationParams.Size,
				Page: request.Query.PaginationParams.Page,
			}, uint(totalResults))
		if err != nil {
			log.Err(err).Msg("failed to generate pagination links")
			goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
			return
		}

		ja.Links = (*jsonapi.Links)(links)
		ja.Meta = &jsonapi.Meta{
			"totalResults": totalResults,
			"totalPages":   uint32(math.Ceil(float64(totalResults) / float64(request.Query.PaginationParams.Size))),
		}
	}

	// Return JSONAPI
	internal.WriteJSONAPIResponse(ctx, jsonAPIResponse, http.StatusOK, nil)
}

// bindRequest bind the request to the Request struct.
func bindRequest(ctx *goskell.Context, defaultPageSize, maxPageSize uint) (*ListInput, error) {
	var (
		request ListInput
		err     error
	)
	request.Query.PaginationParams, err = pagination.GetPagination(ctx, defaultPageSize, maxPageSize)
	if err != nil {
		log.Error().Err(err).Msg("invalid pagination parameters")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusBadRequest), err, http.StatusBadRequest)
		return nil, err
	}

	//Filtering
	request.Match = ctx.QueryMap("match")
	return &request, nil
}
