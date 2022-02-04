package keys

import (
	"context"
	"fmt"
	"github.com/antihax/optional"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-api-key-manager/internal/config"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-sdk-go"
	"net/http"
)

//TODO add unittests once we have the expiration date as well

//Handler handles keys requests
type Handler struct {
	tykClient      *tyk.APIClient
	keysRepository RepositoryInterface
	policies       []string
}

//NewHandler constructor
func NewHandler(tykClient *tyk.APIClient, keysRepository RepositoryInterface, policies []string) *Handler {
	return &Handler{tykClient: tykClient, keysRepository: keysRepository, policies: policies}
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
	return NewHandler(tykClient, keysRepository, policies)
}

//HandlePOST simply tries to create new key from the given input and insert it in the database
func (h *Handler) HandlePOST(c *gin.Context) {
	// parse and validate the request
	var body PostKeyRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input parameters"})
		return
	}

	if err := h.validatePolicies(body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
	key := New(tykResponse.KeyHash, body.ActorID, nil)
	err = h.keysRepository.StoreKey(*key)
	if err != nil {
		_, _, err := h.tykClient.KeysApi.DeleteKey(ctx, tykResponse.KeyHash)
		log.WithError(err).Error("could not store key in database, removed key again from tyk")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		if err != nil {
			log.WithError(err).WithField("hash", tykResponse.KeyHash).Error("could not remove key from tyk after error")
		}
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
	metadata := make(map[string]map[string]interface{})
	metadata["actor_id"] = map[string]interface{}{
		"actor_id": body.ActorID,
	}

	return &tyk.AddKeyOpts{SessionState: optional.NewInterface(tyk.SessionState{
		ApplyPolicies: body.Policies,
		Tags:          []string{},
		MetaData:      metadata,
	})}, nil
}

func (h Handler) validatePolicies(body PostKeyRequest) error {
	var policyFound bool
	for _, reqPolicy := range body.Policies {
		policyFound = false
		// check if policy is available
		for _, availablePolicy := range h.policies {
			if reqPolicy == availablePolicy {
				policyFound = true
				break
			}
		}
		// exit condition
		if !policyFound {
			return fmt.Errorf("policy `%s` not available", reqPolicy)
		}
	}
	return nil
}
