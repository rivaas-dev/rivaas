// Package policies defines required methods of the API policies.
package policies

import "gitlab.ci.fdmg.org/ci-api/tyk-sdk-go"

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
