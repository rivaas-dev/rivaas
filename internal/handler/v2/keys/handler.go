// Package keys defines all methods of the API key.
package keys

import (
	"context"

	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/config"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/customers"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/db"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/handler/v2/policies"
	oma "gitlab.ci.fdmg.org/ci-api/oma/pkg/client"
	"gitlab.ci.fdmg.org/ci-api/tyk-sdk-go"
	"go.companyinfo.dev/keycloak"
	"go.temporal.io/sdk/client"
)

// Handler handles keys requests
type Handler struct {
	tykClient       *tyk.APIClient
	temporalClient  client.Client
	omaClient       *oma.Client
	keysRepository  db.DatabaseExecer
	keycloakClient  keycloak.Client
	keycloakConfig  config.KeyCloakConfig
	customerService *customers.Service
	defaultPolicies []*policies.Policy
	defaultPageSize uint
	maxPageSize     uint
}

// New constructs a new Handler.
func New(ctx context.Context,
	temporalClient client.Client,
	tykClient *tyk.APIClient,
	keysRepository db.DatabaseExecer,
	omaClient *oma.Client,
	keycloakClient keycloak.Client,
	keyCloakConfig config.KeyCloakConfig,
	customerService *customers.Service,
	apiKeyDefaults config.APIKeyDefaults,
	pagination config.Pagination) (*Handler, error) {

	// Convert default policy IDs to full policy objects
	pols, err := policies.GetPoliciesByIDs(ctx, tykClient, apiKeyDefaults.Policies)
	if err != nil {
		return nil, err
	}

	return &Handler{
		tykClient:       tykClient,
		temporalClient:  temporalClient,
		omaClient:       omaClient,
		keysRepository:  keysRepository,
		keycloakClient:  keycloakClient,
		keycloakConfig:  keyCloakConfig,
		customerService: customerService,
		defaultPageSize: pagination.DefaultPageSize,
		defaultPolicies: pols,
		maxPageSize:     pagination.MaxPageSize,
	}, nil
}
