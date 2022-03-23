package key

import (
	"context"
	"github.com/antihax/optional"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-api-key-manager/internal/key/response"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-sdk-go"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell/json/problem"
	"net/http"
)

const (
	HashPathName = "key_hash"
)

//HandleGETKey get single key by hash
func (h *Handler) HandleGETKey(c *gin.Context) {
	// parse and validate the request
	hash := c.Param(HashPathName)
	ctx := context.Background()
	dbKey, err := h.keysRepository.GetKeyByHash(hash)
	if err != nil {
		log.WithError(err).Error(DBCommunicationErrorText)
		goskell.ProblemJSON(c, problem.Details{Title: DBCommunicationErrorText, Status: http.StatusInternalServerError})
		return
	}
	if dbKey == nil {
		goskell.ProblemJSON(c, problem.Details{Title: DBKeyNotFoundText, Status: http.StatusNotFound})
		return
	}

	tykResponse, resp, err := h.keysClient.GetKey(ctx, hash, &tyk.GetKeyOpts{Hashed: optional.NewBool(true)})
	// Means the key was probably removed
	if resp.StatusCode == http.StatusNotFound {
		// build key from db response
		key := response.NewKeyDetailsWithGatewayContext(dbKey.ActorID, dbKey.QuotaEndDate, nil, dbKey.Description,
			nil, dbKey.CreatedAt)
		c.JSON(http.StatusOK, key)
		return
	}

	if err != nil {
		log.WithError(err).Error(GatewayCommunicationErrorText)
		goskell.ProblemJSON(c, problem.Details{Title: GatewayCommunicationErrorText, Status: http.StatusInternalServerError})
		return
	}

	q := &tykResponse.QuotaRemaining
	// If the quota is smaller than 0 it means it's unlimited, so we send nil back
	if *q < 0 {
		q = nil
	}
	key := response.NewKeyDetailsWithGatewayContext(dbKey.ActorID, dbKey.QuotaEndDate, q, dbKey.Description,
		tykResponse.ApplyPolicies, dbKey.CreatedAt)

	c.JSON(http.StatusOK, key)
}

//HandleGETKeys list the keys and optionally filter
func (h *Handler) HandleGETKeys(c *gin.Context) {
	// parse and validate the request
	var input GetKeysInput
	_ = c.ShouldBindQuery(&input)
	dbKeys, err := h.keysRepository.GetKeys(input)
	if err != nil || dbKeys == nil {
		log.WithError(err).Error(DBCommunicationErrorText)
		goskell.ProblemJSON(c, problem.Details{Title: DBCommunicationErrorText, Status: http.StatusInternalServerError})
		return
	}
	res := []response.KeyDetails{}
	// response DB records
	for _, k := range dbKeys {
		newK := response.NewKeyDetails(k.Hash, k.ActorID, k.QuotaEndDate, k.Description, k.CreatedAt)
		res = append(res, *newK)
	}

	c.JSON(http.StatusOK, res)
}
