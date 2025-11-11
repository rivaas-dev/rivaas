package customers

import (
	"context"
	"math"
	"net/http"

	"github.com/companyinfo/jsonapi"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/handler/v2/customers/filters"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/handler/v2/pagination"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/headers"
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/keycloak"
	"gitlab.ci.fdmg.org/datacluster/golibs/goot"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"go.opentelemetry.io/otel/attribute"
)

// ListInput represents the request body of the LIST endpoint.
type ListInput struct {
	headers.Authorization

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

	if !h.isAuthorized(ctx, nil) {
		// The appropriate response is already handled in "isAuthorized()"
		return
	}

	// Fetch keys from the database.
	_, span := goot.Span(ctx.Request.Context(), "get_from_database")
	customerGroups, totalResults, err := h.getCustomers(ctx.Request.Context(), request)
	if err != nil {
		goot.EndSpanWithError(span, err, "failed to call database")
		log.Err(err).Msg("error while communicating with DB")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}
	goot.EndSpan(span)

	// Parse the group data
	parsedKeyCloakGroups, err := h.customerService.ListCustomersFromGroups(ctx.Request.Context(), customerGroups)
	if err != nil {
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}

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
	request.Authorization, err = headers.GetAuthorization(ctx)
	if err != nil {
		return nil, err
	}

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

func (h *Handler) getCustomers(ctx context.Context, request *ListInput) (groups []*keycloak.Group, totalResults int, err error) {
	// if the user is not an admin, return only their group
	if !headers.IsAdministrator(request.Authorization.Roles) {
		_, span := goot.Span(ctx, "get_from_keycloak", attribute.String("customer_id", request.Authorization.CustomerUser.CustomerID))
		group, err := h.keycloakClient.GetGroupByID(ctx, request.Authorization.CustomerUser.CustomerID)
		if err != nil {
			goot.EndSpanWithError(span, err, "failed to call keycloak")
			return nil, 0, errors.New("invalid customer id")
		}
		goot.EndSpan(span)

		return []*keycloak.Group{group}, 1, nil
	}

	searchParams := filters.NewSearchParameters(filters.FilterParam{Match: request.Match})

	firstElement := int((request.Query.PaginationParams.Page - 1) * request.Query.PaginationParams.Size)
	maxPageSize := int(request.Query.PaginationParams.Size)

	// Fetch total group count
	_, span := goot.Span(ctx, "get_group_count_from_keycloak")
	t := true
	totalResults, err = h.keycloakClient.GetGroupsCount(ctx, searchParams.Name, &t)
	if err != nil {
		goot.EndSpanWithError(span, err, "failed to call keycloak")
		return nil, 0, err
	}
	goot.EndSpan(span)

	// Fetch the requested page of the groups
	_, span = goot.Span(ctx, "get_from_keycloak")
	groups, err = h.keycloakClient.GetGroupsPaginated(ctx, searchParams.Name, h.keycloakConfig.BrifRepresentation, firstElement, maxPageSize)
	if err != nil {
		goot.EndSpanWithError(span, err, "failed to call keycloak")
		return nil, 0, err
	}
	goot.EndSpan(span)

	return groups, totalResults, nil
}
