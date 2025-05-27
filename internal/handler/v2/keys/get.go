// Package keys defines all methods of the API key.
package keys

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/antihax/optional"
	"github.com/companyinfo/jsonapi"
	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/db"
	"gitlab.ci.fdmg.org/ci-api/tyk-sdk-go"
	"gitlab.ci.fdmg.org/datacluster/golibs/goot"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"go.opentelemetry.io/otel/attribute"
	"net/http"
	"time"
)

// GET handles GET requests on the endpoint.
func (h *Handler) GET(ctx *goskell.Context) {
	// Parse request body.
	var request KeyID
	if err := ctx.ShouldBindUri(&request); err != nil {
		goskell.JsonAPIError(ctx, "input body validation", err, http.StatusBadRequest)
		return
	}

	_, span := goot.Span(ctx.Request.Context(), "get_from_database",
		attribute.String("id", request.ID),
	)
	// Find the key in database.
	dbKey, err := h.keysRepository.GetKey(request.ID)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			goot.EndSpanWithError(span, err, "key not found")
			log.Err(err).Msg("key not found in database")
			goskell.JsonAPIError(ctx, "key not found", err, http.StatusNotFound)
			return
		}
		goot.EndSpanWithError(span, err, "failed to call database")
		log.Err(err).Msg("error while communicating with DB")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}
	goot.EndSpan(span)

	if !h.isAuthorized(ctx, NewKeyActorID(
		dbKey.ActorID,
		dbKey.CreatorID,
	)) {
		// The appropriate response is already handled in "isAuthorized()"
		return
	}

	// Get more info of the key.
	response, err := h.getKeyInfo(ctx, dbKey)
	if err != nil {
		log.Err(err).Msg("error on calling worker")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}
	// convert to JSON API response
	jsonAPIResponse, err := jsonapi.Marshal(response)
	if err != nil {
		log.Err(err).Msg("error on marshaling response")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}

	internal.WriteJSONAPIResponse(ctx, jsonAPIResponse, http.StatusOK, nil)
}

// getKeyInfo gets more info of the key by calling Tyk API.
func (h *Handler) getKeyInfo(ctx *goskell.Context, dbKey *db.Key) (*APIKey, error) {
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

	customerName, err := h.getCustomerName(ctx, dbKey)
	if err != nil {
		return nil, errors.New("customer name not found")
	}

	// build key from db response
	result := APIKey{
		ID:             dbKey.ID,
		CustomerName:   customerName,
		Hash:           dbKey.Hash,
		CreationDate:   dbKey.CreatedAt.Format(time.RFC3339),
		Environment:    dbKey.Environment,
		ActorID:        dbKey.ActorID,
		CreatorID:      dbKey.CreatorID,
		Policies:       tykResponse.ApplyPolicies,
		ExpiresAt:      dbKey.ExpiresAt,
		Quota:          tykResponse.QuotaMax,
		QuotaRemaining: tykResponse.QuotaRemaining,
		Description:    String(dbKey.Description),
		CreatedDate:    dbKey.CreatedAt.Format(time.RFC3339),
		Contact:        Contact(dbKey.Contact),
		Active:         !tykResponse.IsInactive,
		RateLimit: RateLimit{
			Rate: tykResponse.Rate,
			Per:  tykResponse.Per,
		},
		Labels: dbKey.Labels,
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

func (h *Handler) getCustomerName(ctx context.Context, dbKey *db.Key) (string, error) {
	// Get customer data
	_, span := goot.Span(ctx, "get_from_keycloak")
	keycloakGroups, err := h.keycloakClient.GetGroups(ctx, h.keycloakConfig.BrifRepresentation, h.keycloakConfig.First, h.keycloakConfig.Max)
	if err != nil {
		goot.EndSpanWithError(span, err, "failed to call keycloak")
		log.Err(err).Msg("error while communicating with keycloak")
		return "", err
	}

	goot.EndSpan(span)
	return internal.GetCustomerName(keycloakGroups, *dbKey), nil
}
