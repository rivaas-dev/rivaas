// Package keys defines all methods of the API key.
package keys

import (
	"errors"
	"net/http"
	"time"

	"github.com/antihax/optional"
	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/db"
	"gitlab.ci.fdmg.org/ci-api/tyk-sdk-go"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell/json/problem"
)

// GetOutput represents a Key information.
type GetOutput struct {
	ActorID      string            `json:"actor_id"`
	ExpiresAt    string            `json:"expires_at"`
	Quota        int64             `json:"quota"`
	Description  string            `json:"description"`
	Policies     []string          `json:"policies"`
	CreationDate time.Time         `json:"creation_date"`
	Contact      Contact           `json:"contacts,omitempty"`
	Active       bool              `json:"active"`
	RateLimit    RateLimit         `json:"rate_limit"`
	Environment  ApikeyEnvironment `json:"environment"`
	Labels       map[string]string `json:"labels"`
}

// GET handles GET requests on the endpoint.
func (h *Handler) GET(ctx *goskell.Context) {
	// Parse request body.
	var request KeyID
	if err := ctx.ShouldBindUri(&request); err != nil {
		goskell.ProblemJSON(
			ctx,
			problem.Details{
				Title:  http.StatusText(http.StatusBadRequest),
				Status: http.StatusBadRequest,
			},
		)
		return
	}

	// Find the key in database.
	dbKey, err := h.keysRepository.GetKey(request.Hash)
	if err != nil {
		log.Err(err).Msg("error while communicating with DB")
		goskell.ProblemJSON(ctx, problem.Details{Status: http.StatusInternalServerError})
		return
	}
	if dbKey == nil {
		log.Err(err).Msg("key not found in database")
		goskell.ProblemJSON(ctx, problem.Details{Status: http.StatusNotFound})
		return
	}

	// Get more info of the key.
	response, err := h.getKeyInfo(ctx, dbKey)
	if err != nil {
		log.Err(err).Msg("error on calling worker")
		goskell.ProblemJSON(ctx, problem.Details{Status: http.StatusInternalServerError})
	}

	ctx.JSON(http.StatusCreated, response)
}

// getKeyInfo gets more info of the key by calling Tyk API.
func (h *Handler) getKeyInfo(ctx *goskell.Context, dbKey *db.Key) (*GetOutput, error) {
	// Get Key info.
	tykResponse, resp, err := h.tykClient.KeysApi.GetKey(
		ctx,
		dbKey.Hash,
		&tyk.GetKeyOpts{Hashed: optional.NewBool(true)},
	)
	if err != nil {
		return nil, err
	}
	// Means the key was probably removed
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("key not found")
	}

	// build key from db response
	result := GetOutput{
		ActorID:      dbKey.ActorID,
		Policies:     tykResponse.ApplyPolicies,
		Quota:        tykResponse.QuotaMax,
		CreationDate: dbKey.CreatedAt,
		Contact:      Contact(dbKey.Contact),
		Active:       !tykResponse.IsInactive,
		RateLimit: RateLimit{
			Rate: uint(tykResponse.Rate),
			Per:  uint(tykResponse.Per),
		},
		Environment: dbKey.Environment,
		Labels:      dbKey.Labels,
	}
	if tykResponse.Expires > 0 {
		result.ExpiresAt = time.Unix(tykResponse.Expires, 0).UTC().Format("2006-01-02")
	} else {
		result.ExpiresAt = "0"
	}
	if dbKey.Description != nil {
		result.Description = *dbKey.Description
	}

	return &result, nil
}
