package handler

import (
	"context"
	"github.com/antihax/optional"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-api-key-manager/internal/config"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-sdk-go"
	"net/http"
)

type KeysHandler struct {
	configuration *config.Tyk
}

func NewKeysHandlerWithConfiguration(configuration *config.Tyk) (*KeysHandler, error) {
	return &KeysHandler{configuration: configuration}, nil
}

func (h *KeysHandler) HandlePOST(c *gin.Context) {
	// parse and validate the request
	var body PostKeyRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invaid input parameters"})
		return
	}

	// prepare the request for Tyk
	ctx := context.Background()
	options, err := h.buildOptions(body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// ask Tyk to do its job
	tykClient, err := h.createClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	tykResponse, _, err := tykClient.KeysApi.AddKey(ctx, options)
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

func (h *KeysHandler) IsReady() bool {
	//TODO: move this to a separate handler.
	return true
}

func (h *KeysHandler) createClient() (*tyk.APIClient, error) {
	tykClient := tyk.NewAPIClient(&tyk.Configuration{
		Host:          h.configuration.Endpoint,
		Scheme:        h.configuration.Scheme,
		DefaultHeader: map[string]string{"x-tyk-authorization": h.configuration.Secret},
		Debug:         h.configuration.Debug,
	})

	return tykClient, nil
}

type PostKeyRequest struct {
	Policies []string `json:"policies"`
	URN      string   `json:"urn" binding:"required"`
}

type PostKeyResponse struct {
	ID   string `json:"key"`
	Hash string `json:"hash"`
}

func (h KeysHandler) buildOptions(body PostKeyRequest) (*tyk.AddKeyOpts, error) {
	platform, err := resolvePlatform(body.URN)
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
