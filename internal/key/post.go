package key

import (
	"context"
	"github.com/antihax/optional"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/key/request/post"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/key/response"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-sdk-go"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell/json/problem"
	"net/http"
	"time"
)

// HandlePOST simply tries to create new key from the given input and insert it in the database
func (h *Handler) HandlePOST(c *gin.Context) {
	// parse and validate the request
	var body post.Post
	if err := c.ShouldBindJSON(&body); err != nil {
		goskell.ProblemJSON(c, problem.Details{Title: http.StatusText(http.StatusBadRequest), Status: http.StatusBadRequest})
		return
	}

	if err := h.postReqValidator.ValidatePost(c, body); err != nil {
		goskell.ProblemJSON(c, problem.Details{Title: err.Error(), Status: http.StatusBadRequest})
		return
	}

	// prepare the request for Tyk
	ctx := context.Background()
	options := h.buildPostKeyOptions(body)

	tykResponse, _, err := h.keysClient.AddKey(ctx, options)
	if err != nil {
		log.WithError(err).Error(CreateKeyGeneralErrorText)
		goskell.ProblemJSON(c, problem.Details{Title: err.Error(), Status: http.StatusInternalServerError})
		return
	}
	var quotaEndDate *time.Time
	if body.QuotaEndDate != nil {
		quotaEndDate = &body.QuotaEndDate.Time
	}

	key := New(tykResponse.KeyHash, body.ActorID, quotaEndDate, body.Description)
	err = h.keysRepository.StoreKey(*key)
	if err != nil {
		log.WithError(err).Error("could not store key in database, removing again from tyk..")
		goskell.ProblemJSON(c, problem.Details{Title: err.Error(), Status: http.StatusInternalServerError})
		_, _, err := h.keysClient.DeleteKey(ctx, tykResponse.KeyHash, &tyk.DeleteKeyOpts{Hashed: optional.NewBool(
			true)})

		if err != nil {
			log.WithError(err).WithField("hash", tykResponse.KeyHash).Error("could not remove key from tyk after error")
		}
		return
	}

	c.JSON(http.StatusCreated, response.Post{
		ID:           tykResponse.Key,
		Hash:         tykResponse.KeyHash,
		QuotaEndDate: quotaEndDate,
	})
}

func (h Handler) buildPostKeyOptions(body post.Post) *tyk.AddKeyOpts {
	metadata := make(map[string]interface{})
	metadata["actor_id"] = body.ActorID

	stateObj := tyk.SessionState{
		ApplyPolicies: body.Policies,
		Tags:          []string{},
		MetaData:      metadata,
	}

	stateObj.Expires = 0 // never expire!
	// make sure the quota never renews
	stateObj.QuotaRenews = -1
	stateObj.QuotaRenewalRate = 0

	stateObj.QuotaRemaining = -1
	stateObj.QuotaMax = -1
	if body.Quota != nil {
		stateObj.QuotaRemaining = *body.Quota
		stateObj.QuotaMax = *body.Quota
	}

	return &tyk.AddKeyOpts{SessionState: optional.NewInterface(stateObj)}
}
