package customers

import (
	"github.com/companyinfo/jsonapi"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/keycloak"
	"gitlab.ci.fdmg.org/datacluster/golibs/goot"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"net/http"
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

	// Fetch
	_, span := goot.Span(ctx.Request.Context(), "get_from_keycloak")
	group, err := h.keycloakClient.GetGroupByID(ctx, request.Path.CustomerID)
	if err != nil {
		goot.EndSpanWithError(span, err, "failed to call keycloak")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}
	goot.EndSpan(span)

	// Parse the group data
	parsedKeyCloakGroups := keycloak.ParseCustomerGroup(group)

	// Build response from groups
	response, err := jsonapi.Marshal(parsedKeyCloakGroups)
	if err != nil {
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}

	// Return JSONAPI
	internal.WriteJSONAPIResponse(ctx, response, http.StatusOK, nil)
}
