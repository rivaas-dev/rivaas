package accounts

import (
	"errors"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal"
	"net/http"

	"github.com/companyinfo/jsonapi"
	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/customers"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
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

	// Get and parse the group data, the spans are created inside the GetAccountExtended function
	account, err := h.customerService.GetAccountExtended(ctx.Request.Context(), request.Path.CustomerID, request.Path.AccountID)
	if errors.Is(err, customers.ErrPricingPlanNotFound) {
		log.Error().Err(err).
			Str("customerID", request.Path.CustomerID).
			Str("accountID", request.Path.AccountID).
			Msg("pricing plan not found for account")
		goskell.JsonAPIError(ctx, "subscription information unavailable",
			errors.New("subscription plan is not properly configured"),
			http.StatusUnprocessableEntity)
		return
	}
	if err != nil {
		log.Error().Err(err).
			Str("customerID", request.Path.CustomerID).
			Str("accountID", request.Path.AccountID).
			Msg("failed to convert groups to customer")
		goskell.JsonAPIError(ctx, "failed to create a response", err, http.StatusInternalServerError)
		return
	}

	// Build response from groups
	response, err := jsonapi.Marshal(account)
	if err != nil {
		log.Error().Err(err).
			Str("customerID", request.Path.CustomerID).
			Str("accountID", request.Path.AccountID).
			Msg("failed to marshal account response")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}

	// Return JSONAPI
	internal.WriteJSONAPIResponse(ctx, response, http.StatusOK, nil)
}
