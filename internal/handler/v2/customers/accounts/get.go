package accounts

import (
	"encoding/json"
	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
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

	// Get and parse the group data, the spans are created inside the GetAccountExtended function
	account, err := h.customerService.GetAccountExtended(ctx.Request.Context(), request.Path.CustomerID, request.Path.AccountID)
	if err != nil {
		log.Error().Err(err).
			Str("customerID", request.Path.CustomerID).
			Str("accountID", request.Path.AccountID).
			Msg("failed to convert groups to customer")
		goskell.JsonAPIError(ctx, "failed to create a response", err, http.StatusInternalServerError)
		return
	}

	// Build response from groups
	response, err := json.Marshal(account)
	if err != nil {
		log.Error().Err(err).
			Str("customerID", request.Path.CustomerID).
			Str("accountID", request.Path.AccountID).
			Msg("failed to marshal account response")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}

	// Return JSONAPI
	internal.WriteResponse(ctx, response, http.StatusOK, nil)
}
