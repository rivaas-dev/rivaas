// Package listpolicy returns list of policies.
package listpolicy

import (
	"context"
	"net/http"
	"sort"

	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/tyk-sdk-go"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell/json/problem"
)

// Handler handles keys requests
type Handler struct {
	tykClient *tyk.APIClient
}

// New constructs a new Handler.
func New(tykClient *tyk.APIClient) *Handler {
	return &Handler{
		tykClient: tykClient,
	}
}

// Handle handles endpoint requests.
func (h *Handler) Handle(ctx *goskell.Context) {
	// retrieve the policies list
	policies, err := GetPolicies(ctx.Request.Context(), h.tykClient)
	if err != nil {
		log.Err(err).Msg("failed to fetch policies")
		goskell.ProblemJSON(ctx, problem.Details{Status: http.StatusInternalServerError})
		return
	}

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
