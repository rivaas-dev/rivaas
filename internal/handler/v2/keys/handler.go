// Package keys defines all methods of the API key.
package keys

import (
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/config"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/customers"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/db"
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
	apiKeyDefaults  config.APIKeyDefaults
	defaultPageSize uint
	maxPageSize     uint
}

// New constructs a new Handler.
func New(temporalClient client.Client,
	tykClient *tyk.APIClient,
	keysRepository db.DatabaseExecer,
	omaClient *oma.Client,
	keycloakClient keycloak.Client,
	keyCloakConfig config.KeyCloakConfig,
	customerService *customers.Service,
	apiKeyDefaults config.APIKeyDefaults,
	pagination config.Pagination) *Handler {
	return &Handler{
		tykClient:       tykClient,
		temporalClient:  temporalClient,
		omaClient:       omaClient,
		keysRepository:  keysRepository,
		keycloakClient:  keycloakClient,
		keycloakConfig:  keyCloakConfig,
		customerService: customerService,
		defaultPageSize: pagination.DefaultPageSize,
		apiKeyDefaults:  apiKeyDefaults,
		maxPageSize:     pagination.MaxPageSize,
	}
}
