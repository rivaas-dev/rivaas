package accounts

import (
	"encoding/json"
	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal"
	"gitlab.ci.fdmg.org/datacluster/golibs/goot"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"go.opentelemetry.io/otel/attribute"
	"net/http"
)

type GetInput struct {
	Path struct {
		CustomerID string `uri:"customerID" binding:"required"`
		AccountID  string `uri:"accountID" binding:"required"`
	}
}

// GET returns a customer by ID
func (h *Handler) GET(ctx *goskell.Context) {
	var request GetInput
	if err := ctx.BindUri(&request.Path); err != nil {
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusBadRequest), err, http.StatusBadRequest)
		return
	}

	if !h.isAuthorized(ctx, &Customer{
		ID: request.Path.CustomerID,
		Account: Account{
			ID: request.Path.AccountID,
		},
	}) {
		// The appropriate response is already handled in "isAuthorized()"
		return
	}

	_, span := goot.Span(ctx.Request.Context(), "get_group_from_keycloak",
		attribute.String("customerID", request.Path.CustomerID),
	)
	group, err := h.keycloakClient.GetGroupByID(ctx, request.Path.CustomerID)
	if err != nil {
		log.Error().Err(err).
			Str("customerID", request.Path.CustomerID).
			Str("accountID", request.Path.AccountID).
			Msg("failed to retrieve keycloak group")
		goot.EndSpanWithError(span, err, "failed to retrieve keycloak group")
		goskell.JsonAPIError(ctx, "failed to find customer", err, http.StatusInternalServerError)
		return
	}
	goot.EndSpan(span)

	_, span = goot.Span(ctx.Request.Context(), "get_subgroup_from_keycloak",
		attribute.String("accountID", request.Path.AccountID),
	)
	subgroup, err := h.keycloakClient.GetSubGroupByID(*group, request.Path.AccountID)
	if err != nil {
		log.Error().Err(err).
			Str("customerID", request.Path.CustomerID).
			Str("accountID", request.Path.AccountID).
			Msg("failed to retrieve keycloak subgroup")
		goot.EndSpanWithError(span, err, "failed to retrieve keycloak subgroup")
		goskell.JsonAPIError(ctx, "failed to find account", err, http.StatusInternalServerError)
		return
	}
	goot.EndSpan(span)

	//Parse the group data
	_, span = goot.Span(ctx.Request.Context(), "group_to_account")
	account, err := h.customerService.GroupToAccountExtended(group, subgroup)
	if err != nil {
		log.Error().Err(err).
			Str("customerID", request.Path.CustomerID).
			Str("accountID", request.Path.AccountID).
			Msg("failed to convert groups to customer")
		goot.EndSpanWithError(span, err, "failed to convert groups to customer")
		goskell.JsonAPIError(ctx, "failed to create a response", err, http.StatusInternalServerError)
		return
	}
	goot.EndSpan(span)

	// Build response from groups
	response, err := json.Marshal(account)
	if err != nil {
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}

	// Return JSONAPI
	internal.WriteResponse(ctx, response, http.StatusOK, nil)
}
