package key

import (
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-api-key-manager/internal/config"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-api-key-manager/internal/key/api"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-api-key-manager/internal/key/request"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-sdk-go"
)

//Handler handles keys requests
type Handler struct {
	keysClient     api.ClientInterface
	keysRepository RepositoryInterface
	reqValidator   *request.Validator
}

//NewHandler constructor
func NewHandler(tykClient api.ClientInterface, keysRepository RepositoryInterface, policies []string) *Handler {
	validator := request.NewValidator(policies)

	return &Handler{keysClient: tykClient, keysRepository: keysRepository, reqValidator: validator}
}

//NewHandlerFromConfiguration constructor from tyk config
func NewHandlerFromConfiguration(configuration *config.Tyk, keysRepository RepositoryInterface,
	policies []string) *Handler {
	tykClient := tyk.NewAPIClient(&tyk.Configuration{
		Host:          configuration.Endpoint,
		Scheme:        configuration.Scheme,
		DefaultHeader: map[string]string{"x-tyk-authorization": configuration.Secret},
		Debug:         configuration.Debug,
	})
	return NewHandler(tykClient.KeysApi, keysRepository, policies)
}
