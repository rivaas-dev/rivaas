// Package policies defines required methods of the API policies.
package policies

import (
	"context"
	"gitlab.ci.fdmg.org/datacluster/golibs/goot"
	"net/http"
	"sort"

	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/tyk-sdk-go"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell/json/problem"
)

// LIST handles GET requests on the endpoint to get list of policies.
func (h *Handler) LIST(ctx *goskell.Context) {
	// retrieve the policies list
	_, span := goot.Span(ctx.Request.Context(), "get_from_tyk")
	policies, err := GetPolicies(ctx.Request.Context(), h.tykClient)
	if err != nil {
		goot.EndSpanWithError(span, err, "failed to fetch policies")
		log.Err(err).Msg("failed to fetch policies")
		goskell.ProblemJSON(ctx, problem.Details{Status: http.StatusInternalServerError})
		return
	}
	goot.EndSpan(span)

	ctx.JSON(http.StatusOK, policies)
}

// GetPolicies fetch list of policies from Tyk server.
func GetPolicies(ctx context.Context, tykClient *tyk.APIClient) ([]string, error) {
	policies, _, err := tykClient.PoliciesApi.ListPolicies(ctx)
	if err != nil {
		return nil, err
	}

	res := make([]string, 0, len(policies))
	for _, p := range policies {
		res = append(res, p.Id)
	}

	sort.Strings(res)
	return res, err
}
