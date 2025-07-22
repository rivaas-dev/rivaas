package accounts

import (
	"github.com/companyinfo/jsonapi"
	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/handler/v2/pagination"
	keycloakAccounts "gitlab.ci.fdmg.org/ci-api/admin-api/internal/keycloak"
	"gitlab.ci.fdmg.org/datacluster/golibs/goot"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"math"
	"net/http"
)

type ListAccountsInput struct {
	Path struct {
		CustomerID string `uri:"customerID" binding:"required"`
	}
	Query struct {
		PaginationParams internal.PaginationParams
	}
}

// LIST handles GET requests on the endpoint.
func (h *Handler) LIST(ctx *goskell.Context) {
	// Parse GET request.
	var request ListAccountsInput
	err := ctx.BindUri(&request.Path)
	if err != nil {
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusBadRequest), err, http.StatusBadRequest)
		return
	}

	if !h.isAuthorized(ctx, &Customer{ID: request.Path.CustomerID}) {
		// The appropriate response is already handled in "isAuthorized()"
		return
	}

	request.Query.PaginationParams, err = pagination.GetPagination(ctx, h.defaultPageSize, h.maxPageSize)
	if err != nil {
		log.Error().Err(err).Str("customerID", request.Path.CustomerID).Msg("invalid pagination parameters")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusBadRequest), err, http.StatusBadRequest)
		return
	}

	first := int((request.Query.PaginationParams.Page - 1) * request.Query.PaginationParams.Size)
	max := int(request.Query.PaginationParams.Size)

	// Fetch total group count
	_, span := goot.Span(ctx.Request.Context(), "get_group_count_from_keycloak")
	totalResults, err := h.keycloakClient.GetSubGroupsCount(ctx, request.Path.CustomerID)
	if err != nil {
		goot.EndSpanWithError(span, err, "failed to call keycloak")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}
	goot.EndSpan(span)

	// Fetch group
	_, span = goot.Span(ctx.Request.Context(), "get_from_keycloak")
	group, err := h.keycloakClient.GetGroupByID(ctx, request.Path.CustomerID)
	if err != nil {
		goot.EndSpanWithError(span, err, "failed to call keycloak to get a group")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}
	goot.EndSpan(span)

	// Fetch subgroups
	_, span = goot.Span(ctx.Request.Context(), "get_from_keycloak")
	subgroups, err := h.keycloakClient.GetSubGroupsPaginated(ctx, request.Path.CustomerID, h.keycloakConfig.BrifRepresentation, first, max)
	if err != nil {
		goot.EndSpanWithError(span, err, "failed to call keycloak to get subgroups")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}
	goot.EndSpan(span)

	// Parse the group data
	parsedKeyCloakGroups := keycloakAccounts.ParseAccountGroups(group, subgroups)

	// Build response from groups
	response, err := jsonapi.Marshal(parsedKeyCloakGroups)
	if err != nil {
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}

	//Adds the pagination links to the end of the response
	if ja, ok := response.(*jsonapi.ManyPayload); ok {
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
	internal.WriteJSONAPIResponse(ctx, response, http.StatusOK, nil)
}
