package keys

import (
	"context"
	"github.com/antihax/optional"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-api-key-manager/internal/config"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-api-key-manager/internal/urn"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-sdk-go"
	"net/http"
)

//Handler handles keys requests
type Handler struct {
	tykClient *tyk.APIClient
}

//NewHandler constructor
func NewHandler(tykClient *tyk.APIClient) *Handler {
	return &Handler{tykClient: tykClient}
}

//NewHandlerFromConfiguration constructor from tyk config
func NewHandlerFromConfiguration(configuration *config.Tyk) *Handler {
	tykClient := tyk.NewAPIClient(&tyk.Configuration{
		Host:          configuration.Endpoint,
		Scheme:        configuration.Scheme,
		DefaultHeader: map[string]string{"x-tyk-authorization": configuration.Secret},
		Debug:         configuration.Debug,
	})
	return NewHandler(tykClient)
}

//HandlePOST simply tries to create new key from the given input
func (h *Handler) HandlePOST(c *gin.Context) {
	// parse and validate the request
	var body PostKeyRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invaid input parameters"})
		return
	}

	// prepare the request for Tyk
	ctx := context.Background()
	options, err := h.buildKeyOptions(body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	tykResponse, _, err := h.tykClient.KeysApi.AddKey(ctx, options)
	if err != nil {
		log.WithError(err).Error("could not create key")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, PostKeyResponse{
		ID:   tykResponse.Key,
		Hash: tykResponse.KeyHash,
	})
}

func (h *Handler) IsReady() bool {
	//TODO: move this to a separate handler.
	return true
}

func (h Handler) buildKeyOptions(body PostKeyRequest) (*tyk.AddKeyOpts, error) {
	platform, err := urn.ResolvePlatform(body.URN)
	if err != nil {
		return nil, err
	}

	metadata := make(map[string]map[string]interface{})
	metadata["urn"] = map[string]interface{}{
		"urn": body.URN,
	}

	return &tyk.AddKeyOpts{SessionState: optional.NewInterface(tyk.SessionState{
		ApplyPolicies: body.Policies,
		Tags:          []string{*platform},
		MetaData:      metadata,
	})}, nil
}
