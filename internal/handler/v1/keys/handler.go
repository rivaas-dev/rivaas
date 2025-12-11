// Package keys defines all methods of the API key.
package keys

import (
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/db"
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/customer"
	oma "gitlab.ci.fdmg.org/ci-api/oma/pkg/client"
	"gitlab.ci.fdmg.org/ci-api/tyk-sdk-go"
	"go.temporal.io/sdk/client"
)

// Handler handles keys requests
type Handler struct {
	tykClient      *tyk.APIClient
	temporalClient client.Client
	omaClient      *oma.Client
	keysRepository db.DatabaseExecer
	customerClient customer.CustomerClient
}

// New constructs a new Handler.
func New(temporalClient client.Client, tykClient *tyk.APIClient, keysRepository db.DatabaseExecer, omaClient *oma.Client, customerClient customer.CustomerClient) *Handler {
	return &Handler{
		tykClient:      tykClient,
		temporalClient: temporalClient,
		omaClient:      omaClient,
		keysRepository: keysRepository,
		customerClient: customerClient,
	}
}
