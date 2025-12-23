// Package policies defines required methods of the API policies.
package policies

import (
	oma "gitlab.ci.fdmg.org/ci-api/oma/pkg/client"
	"gitlab.ci.fdmg.org/ci-api/tyk-sdk-go"
)

// Handler handles keys requests
type Handler struct {
	tykClient *tyk.APIClient
	omaClient *oma.Client
}

// New constructs a new Handler.
func New(tykClient *tyk.APIClient, oma *oma.Client) *Handler {
	return &Handler{
		tykClient: tykClient,
		omaClient: oma,
	}
}
