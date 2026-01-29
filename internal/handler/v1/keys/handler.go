// Package keys defines all methods of the API key.
package keys

import (
	"strings"

	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/db"
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/customer"
	oma "gitlab.ci.fdmg.org/ci-api/oma/pkg/client"
	"gitlab.ci.fdmg.org/ci-api/tyk-sdk-go"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"go.temporal.io/sdk/client"
)

const (
	deRegionHeader = "X-CI-Region"
	deRegion       = "de"
)

// Handler handles keys requests
type Handler struct {
	tykClient      *tyk.APIClient
	temporalClient client.Client
	omaClient      *oma.Client
	keysRepository db.DatabaseExecer
	customerClient customer.CustomerClient
	// DE Clients
	keysRepositoryDE db.DatabaseExecer
	tykClientDE      *tyk.APIClient
}

// New constructs a new Handler.
func New(temporalClient client.Client, tykClient *tyk.APIClient, keysRepository db.DatabaseExecer,
	omaClient *oma.Client, customerClient customer.CustomerClient,
	keysRepositoryDE db.DatabaseExecer, tykClientDE *tyk.APIClient) *Handler {
	return &Handler{
		tykClient:        tykClient,
		temporalClient:   temporalClient,
		omaClient:        omaClient,
		keysRepository:   keysRepository,
		customerClient:   customerClient,
		keysRepositoryDE: keysRepositoryDE,
		tykClientDE:      tykClientDE,
	}
}

// getRegion returns the normalized region value ("de" or empty string).
func (h *Handler) getRegion(ctx *goskell.Context) string {
	region := strings.ToLower(strings.TrimSpace(ctx.Request.Header.Get(deRegionHeader)))
	if region == deRegion {
		return deRegion
	}
	return ""
}

// getTykClient returns the appropriate Tyk client based on the given region.
func (h *Handler) getTykClient(region string) *tyk.APIClient {
	if strings.ToLower(strings.TrimSpace(region)) == deRegion {
		return h.tykClientDE
	}
	return h.tykClient
}

// getKeysRepository returns the appropriate keys repository based on the given region.
func (h *Handler) getKeysRepository(region string) db.DatabaseExecer {
	if strings.ToLower(strings.TrimSpace(region)) == deRegion {
		return h.keysRepositoryDE
	}
	return h.keysRepository
}
