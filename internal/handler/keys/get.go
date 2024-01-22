// Package keys defines all methods of the API key.
package keys

import (
	"encoding/json"
	"errors"
	"fmt"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal"
	"gitlab.ci.fdmg.org/datacluster/golibs/goot"
	"go.opentelemetry.io/otel/attribute"
	"net/http"
	"time"

	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/date"

	"github.com/antihax/optional"
	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/db"
	"gitlab.ci.fdmg.org/ci-api/tyk-sdk-go"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell/json/problem"
)

// GetOutput represents a Key information.
type GetOutput struct {
	ID             string            `json:"id"`
	CustomerName   string            `json:"customer_name"`
	Hash           string            `json:"hash"`
	ActorID        string            `json:"actor_id"`
	CreatorID      string            `json:"creator_id"`
	ExpiresAt      *date.Date        `json:"expires_at"`
	Quota          int64             `json:"quota"`
	QuotaRemaining int64             `json:"quota_remaining"`
	Description    string            `json:"description"`
	Policies       []string          `json:"policies"`
	CreationDate   time.Time         `json:"creation_date"`
	Contact        Contact           `json:"contacts,omitempty"`
	Active         bool              `json:"active"`
	RateLimit      RateLimit         `json:"rate_limit"`
	Environment    ApikeyEnvironment `json:"environment"`
	Labels         map[string]string `json:"labels"`
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

	_, span := goot.Span(ctx.Request.Context(), "get_from_database",
		attribute.String("id", request.ID),
	)
	// Find the key in database.
	dbKey, err := h.keysRepository.GetKey(request.ID)
	if err != nil {
		goot.EndSpanWithError(span, err, "failed to call database")
		log.Err(err).Msg("error while communicating with DB")
		goskell.ProblemJSON(
			ctx,
			problem.Details{
				Status: http.StatusInternalServerError,
				Title:  http.StatusText(http.StatusInternalServerError),
			},
		)
		return
	}
	if dbKey == nil {
		goot.EndSpanWithError(span, err, "key not found")
		log.Err(err).Msg("key not found in database")
		goskell.ProblemJSON(
			ctx,
			problem.Details{
				Status: http.StatusNotFound,
				Title:  http.StatusText(http.StatusNotFound),
			},
		)
		return
	}
	goot.EndSpan(span)

	if !h.isAuthorized(ctx, dbKey) {
		// The appropriate response is already handled in "isAuthorized()"
		return
	}

	// Get more info of the key.
	response, err := h.getKeyInfo(ctx, dbKey)
	if err != nil {
		log.Err(err).Msg("error on calling worker")
		goskell.ProblemJSON(
			ctx,
			problem.Details{
				Status: http.StatusInternalServerError,
				Title:  http.StatusText(http.StatusInternalServerError),
			},
		)

		return
	}

	ctx.JSON(http.StatusOK, response)
}

// getKeyInfo gets more info of the key by calling Tyk API.
func (h *Handler) getKeyInfo(ctx *goskell.Context, dbKey *db.Key) (*GetOutput, error) {
	// Get Key info.
	_, span := goot.Span(ctx.Request.Context(), "get_from_tyk",
		attribute.String("hash", dbKey.Hash),
	)
	tykResponse, resp, err := h.tykClient.KeysApi.GetKey(
		ctx,
		dbKey.Hash,
		&tyk.GetKeyOpts{Hashed: optional.NewBool(true)},
	)
	if err != nil {
		goot.EndSpanWithError(span, err, "failed to call tyk")
		return nil, err
	}
	// Means the key was probably removed
	if resp.StatusCode != http.StatusOK {
		goot.EndSpanWithError(span, err, fmt.Sprintf("key not found: status code %d", resp.StatusCode))
		return nil, errors.New("key not found")
	}
	goot.EndSpan(span)

	// Get customer data
	_, span = goot.Span(ctx.Request.Context(), "get_from_keycloak")
	keycloakGroups, err := h.keycloakClient.GetGroups(ctx, h.keycloakConfig.BrifRepresentation, h.keycloakConfig.First, h.keycloakConfig.Max)
	if err != nil {
		goot.EndSpanWithError(span, err, "failed to call keycloak")
		log.Err(err).Msg("error while communicating with keycloak")
	}
	goot.EndSpan(span)

	// build key from db response
	result := GetOutput{
		ID:             dbKey.ID,
		CustomerName:   internal.GetCustomerName(keycloakGroups, *dbKey),
		Hash:           dbKey.Hash,
		ActorID:        dbKey.ActorID,
		CreatorID:      dbKey.CreatorID,
		ExpiresAt:      dbKey.ExpiresAt,
		Policies:       tykResponse.ApplyPolicies,
		Quota:          tykResponse.QuotaMax,
		QuotaRemaining: tykResponse.QuotaRemaining,
		CreationDate:   dbKey.CreatedAt,
		Contact:        Contact(dbKey.Contact),
		Active:         !tykResponse.IsInactive,
		Environment:    dbKey.Environment,
		Labels:         dbKey.Labels,
	}

	if rateLimit, ok := dbKey.Metadata["rate_limit"]; ok {
		jsonData, _ := json.Marshal(rateLimit)
		var rateLimit RateLimit
		err := json.Unmarshal(jsonData, &rateLimit)
		if err != nil {
			return nil, err
		}
		result.RateLimit = RateLimit{
			Rate: rateLimit.Rate,
			Per:  rateLimit.Per,
		}
	}

	if dbKey.Description != nil {
		result.Description = *dbKey.Description
	}

	return &result, nil
}
