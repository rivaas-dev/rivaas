package customers

import (
	"net/http"

	"github.com/companyinfo/jsonapi"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal"
	"gitlab.ci.fdmg.org/datacluster/golibs/goot"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
)

type GetInput struct {
	Path struct {
		CustomerID string `uri:"customerID" binding:"required"`
	}
}

// GET returns a customer by ID
func (h *Handler) GET(ctx *goskell.Context) {
	var request GetInput
	if err := ctx.BindUri(&request.Path); err != nil {
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusBadRequest), err, http.StatusBadRequest)
		return
	}

	if !h.isAuthorized(ctx, &Customer{ID: request.Path.CustomerID}) {
		// The appropriate response is already handled in "isAuthorized()"
		return
	}

	// Get customer using the new service method
	_, span := goot.Span(ctx.Request.Context(), "get_customer")
	customer, err := h.customerService.GetCustomer(ctx.Request.Context(), request.Path.CustomerID)
	if err != nil {
		goot.EndSpanWithError(span, err, "failed to get customer")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}
	goot.EndSpan(span)

	// Build response from customer
	response, err := jsonapi.Marshal(customer)
	if err != nil {
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}

	// Return JSONAPI
	internal.WriteJSONAPIResponse(ctx, response, http.StatusOK, nil)
}
