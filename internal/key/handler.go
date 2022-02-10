package key

import (
	"context"
	"github.com/antihax/optional"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-api-key-manager/internal/config"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-api-key-manager/internal/key/api"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-api-key-manager/internal/key/request"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-api-key-manager/internal/key/response"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-sdk-go"
	"net/http"
	"time"
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

//HandlePOST simply tries to create new key from the given input and insert it in the database
func (h *Handler) HandlePOST(c *gin.Context) {
	// parse and validate the request
	var body request.Post
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input parameters"})
		return
	}

	if err := h.reqValidator.ValidatePost(body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// prepare the request for Tyk
	ctx := context.Background()
	options := h.buildKeyOptions(body)

	tykResponse, _, err := h.keysClient.AddKey(ctx, options)
	if err != nil {
		log.WithError(err).Error("could not create key")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var expiration *time.Time
	if body.ExpirationDate != nil {
		expiration = &body.ExpirationDate.Time
	}

	key := New(tykResponse.KeyHash, body.ActorID, expiration)
	err = h.keysRepository.StoreKey(*key)
	if err != nil {
		log.WithError(err).Error("could not store key in database, removing again from tyk..")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		_, _, err := h.keysClient.DeleteKey(ctx, tykResponse.KeyHash)

		if err != nil {
			log.WithError(err).WithField("hash", tykResponse.KeyHash).Error("could not remove key from tyk after error")
		}
		return
	}

	c.JSON(http.StatusCreated, response.Post{
		ID:             tykResponse.Key,
		Hash:           tykResponse.KeyHash,
		ExpirationDate: expiration,
	})
}

func (h *Handler) IsReady() bool {
	//TODO: move this to a separate handler.
	return true
}

func (h Handler) buildKeyOptions(body request.Post) *tyk.AddKeyOpts {
	metadata := make(map[string]interface{})
	metadata["actor_id"] = body.ActorID
	var expires int64
	if body.ExpirationDate != nil {
		expires = body.ExpirationDate.Unix()
	}

	return &tyk.AddKeyOpts{SessionState: optional.NewInterface(tyk.SessionState{
		ApplyPolicies: body.Policies,
		Tags:          []string{},
		MetaData:      metadata,
		Expires:       expires,
	})}
}
