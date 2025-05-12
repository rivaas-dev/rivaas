// Package policies defines required methods of the API policies.
package policies

import (
	"context"
	"github.com/google/jsonapi"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal"
	"gitlab.ci.fdmg.org/datacluster/golibs/goot"
	"net/http"
	"sort"

	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/tyk-sdk-go"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
)

type Policy struct {
	ID   string `jsonapi:"primary,policies"`
	Name string `jsonapi:"attr,name"`
}

// LIST handles GET requests on the endpoint to get list of policies.
func (h *Handler) LIST(ctx *goskell.Context) {
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

// GetPolicies fetch list of policies from Tyk server.
func GetPolicies(ctx context.Context, tykClient *tyk.APIClient) ([]*Policy, error) {
	policies, _, err := tykClient.PoliciesApi.ListPolicies(ctx)
	if err != nil {
		return nil, err
	}

	res := make([]*Policy, 0, len(policies))
	for _, p := range policies {
		res = append(res, &Policy{
			ID:   p.Id,
			Name: p.Name,
		})
	}

	sort.Slice(res, func(i, j int) bool {
		return res[i].ID < res[j].ID
	})
	return res, err
}
