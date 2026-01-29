// Package policies defines required methods of the API policies.
package policies

import (
	"gitlab.ci.fdmg.org/ci-api/tyk-sdk-go"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"strings"
)

const (
	deRegionHeader = "X-CI-Region"
	deRegion       = "de"
)

// Handler handles keys requests
type Handler struct {
	tykClient   *tyk.APIClient
	tykClientDE *tyk.APIClient
}

// New constructs a new Handler.
func New(tykClient *tyk.APIClient, tykClientDE *tyk.APIClient) *Handler {
	return &Handler{
		tykClient:   tykClient,
		tykClientDE: tykClientDE,
	}
}

// getTykClient returns the appropriate Tyk client based on the given region in the X-CI-Region header.
func (h *Handler) getTykClient(ctx *goskell.Context) *tyk.APIClient {
	if strings.ToLower(strings.TrimSpace(ctx.Request.Header.Get(deRegionHeader))) == deRegion {
		return h.tykClientDE
	}
	return h.tykClient
}
