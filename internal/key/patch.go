package key

import (
	"context"
	"errors"
	"fmt"
	"github.com/antihax/optional"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-api-key-manager/internal/key/request/patch"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-api-key-manager/internal/key/response"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-sdk-go"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell/json/problem"
	"gorm.io/gorm"
	"net/http"
)

const (
	tykStatusOK = "ok"
)

//HandlePATCHKey update single key by hash
func (h *Handler) HandlePATCHKey(c *gin.Context) {
	// parse and validate the request
	typedPatch, err := h.getPatchInputOrFinishRequest(c)
	if err != nil {
		// the getPatchInputOrFinishRequest should have handled the response in case of error
		return
	}

	hash := c.Param(HashPathName)
	ctx := context.Background()
	dbKey, err := h.patchKeyInDbOrFinishRequest(c, hash, typedPatch)
	if err != nil {
		// the patchKeyInDbOrFinishRequest should have handled the response in case of error
		return
	}

	newState, err := h.updateKeyOrFinishRequest(c, ctx, hash, typedPatch)
	if err != nil {
		// TODO: Might have to revert patch here, it's a bit overkill to implement it for now
		// the getKeyFromClientOrFinishRequest should have handled the response in case of error
		return
	}

	key := response.NewKeyDetailsWithGatewayContext(dbKey.ActorID, dbKey.QuotaEndDate, &newState.QuotaRemaining,
		dbKey.Description, newState.ApplyPolicies, dbKey.CreatedAt)

	c.JSON(http.StatusOK, key)
}

//updateKeyOrFinishRequest function to improve readability
func (h *Handler) updateKeyOrFinishRequest(c *gin.Context, ctx context.Context, keyHash string,
	patch *patch.Typed) (*tyk.SessionState, error) {
	tykResponse, resp, err := h.keysClient.GetKey(ctx, keyHash, &tyk.GetKeyOpts{Hashed: optional.NewBool(true)})
	if err != nil {
		log.WithError(err).Error(GatewayCommunicationErrorText)
		goskell.ProblemJSON(c, problem.Details{Title: GatewayCommunicationErrorText, Status: http.StatusInternalServerError})
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		goskell.ProblemJSON(c, problem.Details{Title: GatewayKeyNotFoundText, Status: http.StatusNotFound})
		return nil, GatewayKeyNotFoundError
	}

	newState := h.buildPUTKeyOptions(patch, tykResponse)
	updateKeyResponse, _, err := h.keysClient.UpdateKey(ctx, keyHash, &tyk.UpdateKeyOpts{
		Hashed:       optional.NewBool(true),
		SessionState: optional.NewInterface(newState),
	})

	if err != nil {
		log.WithError(err).Error(GatewayCommunicationErrorText)
		goskell.ProblemJSON(c, problem.Details{Title: GatewayCommunicationErrorText, Status: http.StatusInternalServerError})
		return nil, err
	}

	if updateKeyResponse.Status != tykStatusOK {
		log.WithError(fmt.Errorf("non-ok result code while updating key %s", keyHash)).Error("error while updating key in gateway")
		goskell.ProblemJSON(c, problem.Details{Title: GatewayCommunicationErrorText, Status: http.StatusInternalServerError})
		return nil, GatewayCommError
	}

	return &newState, nil
}

//getPatchInputOrFinishRequest function to improve readability, updates key in DB and returns the key itself
func (h *Handler) patchKeyInDbOrFinishRequest(c *gin.Context, keyHash string, patch *patch.Typed) (*Key,
	error) {
	dbKey, err := h.keysRepository.UpdateKeyByHash(keyHash, patch.ToDBPatchMap())
	if errors.Is(err, gorm.ErrRecordNotFound) {
		goskell.ProblemJSON(c, problem.Details{Title: DBKeyNotFoundText, Status: http.StatusNotFound})
		return nil, DBKeyNotFoundError
	}
	if err != nil {
		log.WithError(err).Error(DBKeyUpdateErrorText)
		goskell.ProblemJSON(c, problem.Details{Title: err.Error(), Status: http.StatusInternalServerError})
		return nil, err
	}

	return dbKey, err
}

//getPatchInputOrFinishRequest function to improve readability
func (h *Handler) getPatchInputOrFinishRequest(c *gin.Context) (*patch.Typed, error) {
	// convert request to map
	var inputMap map[string]interface{}
	if err := c.ShouldBindJSON(&inputMap); err != nil {
		goskell.ProblemJSON(c, problem.Details{Title: http.StatusText(http.StatusBadRequest), Status: http.StatusBadRequest})
		return nil, err
	}
	p, err := patch.BuildTypedPatchFromMap(inputMap)
	if err != nil {
		goskell.ProblemJSON(c, problem.Details{Title: err.Error(), Status: http.StatusBadRequest})
		return nil, err
	}
	if err := h.patchReqValidator.ValidatePatch(inputMap, p); err != nil {
		goskell.ProblemJSON(c, problem.Details{Title: err.Error(), Status: http.StatusBadRequest})
		return nil, err
	}

	return p, nil
}

func (h *Handler) buildPUTKeyOptions(patch *patch.Typed, currentState tyk.SessionState) tyk.SessionState {
	if patch.UpdateQuota && *patch.Quota != -1 {
		// I know this is weird, I didn't come up with this strategy either.
		currentState.QuotaRemaining = currentState.QuotaRemaining + *patch.Quota
		currentState.QuotaMax = currentState.QuotaMax + *patch.Quota
	} else if patch.UpdateQuota && *patch.Quota == -1 {
		currentState.QuotaRemaining = -1
		currentState.QuotaMax = -1
	}
	if patch.UpdatePolicies {
		currentState.AccessRights = map[string]tyk.AccessDefinition{}
		currentState.ApplyPolicies = patch.Policies
	}

	return currentState
}
