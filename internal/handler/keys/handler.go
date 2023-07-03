// Package keys defines all methods of the API key.
package keys

import (
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/db"
	"gitlab.ci.fdmg.org/ci-api/tyk-sdk-go"
	"go.temporal.io/sdk/client"
)

// Handler handles keys requests
type Handler struct {
	tykClient      *tyk.APIClient
	temporalClient client.Client
	keysRepository db.DatabaseExecer
}

// New constructs a new Handler.
func New(temporalClient client.Client, tykClient *tyk.APIClient, keysRepository db.DatabaseExecer) *Handler {
	return &Handler{
		tykClient:      tykClient,
		temporalClient: temporalClient,
		keysRepository: keysRepository,
	}
}
