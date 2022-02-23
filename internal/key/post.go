package key

import (
	"context"
	"github.com/antihax/optional"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-api-key-manager/internal/key/request"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-api-key-manager/internal/key/response"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-sdk-go"
	"net/http"
	"time"
)

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

	key := New(tykResponse.KeyHash, body.ActorID, expiration, body.Description)
	err = h.keysRepository.StoreKey(*key)
	if err != nil {
		log.WithError(err).Error("could not store key in database, removing again from tyk..")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		_, _, err := h.keysClient.DeleteKey(ctx, tykResponse.KeyHash, &tyk.DeleteKeyOpts{Hashed: optional.NewBool(
			true)})

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

func (h Handler) buildKeyOptions(body request.Post) *tyk.AddKeyOpts {
	metadata := make(map[string]interface{})
	metadata["actor_id"] = body.ActorID

	stateObj := tyk.SessionState{
		ApplyPolicies: body.Policies,
		Tags:          []string{},
		MetaData:      metadata,
	}

	if body.ExpirationDate != nil {
		stateObj.Expires = body.ExpirationDate.Unix()
	}
	stateObj.QuotaRemaining = -1
	stateObj.QuotaMax = -1

	if body.Quota != nil {
		stateObj.QuotaRemaining = *body.Quota
		stateObj.QuotaMax = *body.Quota
	}

	return &tyk.AddKeyOpts{SessionState: optional.NewInterface(stateObj)}
}
