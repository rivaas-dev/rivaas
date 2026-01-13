// Package policies defines required methods of the API policies.
package policies

import (
	"net/http"

	"github.com/google/jsonapi"
	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal"
	"gitlab.ci.fdmg.org/datacluster/golibs/goot"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
)

// LIST handles GET requests on the endpoint to get list of policies.
func (h *Handler) LIST(ctx *goskell.Context) {
	if !h.isAuthorized(ctx) {
		// The appropriate response is already handled in "isAuthorized()"
		return
	}

	// retrieve the policies list
	_, span := goot.Span(ctx.Request.Context(), "get_from_tyk")
	policies, err := GetPolicies(ctx.Request.Context(), h.tykClient)
	if err != nil {
		goot.EndSpanWithError(span, err, "failed to fetch policies")
		log.Err(err).Msg("failed to fetch policies")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}
	goot.EndSpan(span)

	_, span = goot.Span(ctx.Request.Context(), "process_response")
	response, err := jsonapi.Marshal(policies)
	if err != nil {
		goot.EndSpanWithError(span, err, "failed to convert response to JSON API")
		log.Err(err).Msg("failed to fetch policies")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}
	goot.EndSpan(span)

	// Return JSONAPI
	internal.WriteJSONAPIResponse(ctx, response, http.StatusOK, nil)
}
