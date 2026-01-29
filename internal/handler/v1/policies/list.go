// Package policies defines required methods of the API policies.
package policies

import (
	"github.com/rs/zerolog/log"
	policiesV2 "gitlab.ci.fdmg.org/ci-api/admin-api/internal/handler/v2/policies"
	"gitlab.ci.fdmg.org/datacluster/golibs/goot"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell/json/problem"
	"net/http"
)

type Policy struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// LIST handles GET requests on the endpoint to get list of policies.
func (h *Handler) LIST(ctx *goskell.Context) {
	// Select region-specific clients based on the header
	tykClient := h.getTykClient(ctx)

	// retrieve the policies list
	_, span := goot.Span(ctx.Request.Context(), "get_from_tyk")
	policies, err := policiesV2.GetPolicies(ctx.Request.Context(), tykClient)
	if err != nil {
		goot.EndSpanWithError(span, err, "failed to fetch policies")
		log.Err(err).Msg("failed to fetch policies")
		goskell.ProblemJSON(ctx, problem.Details{Status: http.StatusInternalServerError})
		return
	}
	goot.EndSpan(span)

	ctx.JSON(http.StatusOK, policies)
}
