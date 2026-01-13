// Package keys defines all methods of the API key.
package keys

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"gitlab.ci.fdmg.org/ci-api/go-pkgs/customer"

	"github.com/antihax/optional"
	"github.com/companyinfo/jsonapi"
	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/date"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/db"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/handler/v2/keys/apikey"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/handler/v2/policies"
	"gitlab.ci.fdmg.org/ci-api/tyk-sdk-go"
	"gitlab.ci.fdmg.org/datacluster/golibs/goot"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"go.opentelemetry.io/otel/attribute"
)

// GET handles GET requests on the endpoint.
func (h *Handler) GET(ctx *goskell.Context) {
	// Parse request body.
	request, err := bindGetRequest(ctx)
	if err != nil {
		goskell.JsonAPIError(ctx, "request validation", err, http.StatusBadRequest)
		return
	}

	_, span := goot.Span(ctx.Request.Context(), "get_from_database",
		attribute.String("id", request.ID),
	)
	// Find the key in database.
	dbKey, err := h.keysRepository.GetKey(request.ID)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		goot.EndSpanWithError(span, err, "failed to call database")
		log.Err(err).Msg("error while communicating with DB")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}
	if errors.Is(err, db.ErrKeyNotFound) || dbKey == nil {
		goot.EndSpanWithError(span, err, "key not found")
		log.Err(err).Msg("key not found in database")
		goskell.JsonAPIError(ctx, "key not found", err, http.StatusNotFound)
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

	// Get more info of the key if the key wasn't deleted
	var tykResponse *tyk.SessionState
	if dbKey.DeletedAt == nil {
		tykResponse, err = h.getTykInfo(ctx, dbKey.Hash)
		if err != nil {
			log.Err(err).Msg("error on calling tyk")
			goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
			return
		}
	}

	customerName, err := h.getCustomerName(ctx, dbKey.ActorID)
	if err != nil {
		log.Err(err).Msg("error on retrieving customer name")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}

	key, err := h.merge(ctx, dbKey, tykResponse, customerName)
	if err != nil {
		log.Err(err).Msg("error on preparing output")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}

	// convert to JSON API response
	jsonAPIResponse, err := jsonapi.Marshal(key)
	if err != nil {
		log.Err(err).Msg("error on marshaling response")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return
	}

	internal.WriteJSONAPIResponse(ctx, jsonAPIResponse, http.StatusOK, nil)
}

// getTykInfo gets more info of the key by calling Tyk API.
func (h *Handler) getTykInfo(ctx *goskell.Context, hash string) (*tyk.SessionState, error) {
	// Get Key info.
	_, span := goot.Span(ctx.Request.Context(), "get_from_tyk",
		attribute.String("hash", hash),
	)
	tykResponse, resp, err := h.tykClient.KeysApi.GetKey(
		ctx,
		hash,
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

	return &tykResponse, nil
}

func (h *Handler) merge(ctx context.Context, dbKey *db.Key, tykResponse *tyk.SessionState, customerName string) (*apikey.APIKey, error) {
	result := apikey.APIKey{
		ID:           dbKey.ID,
		Name:         dbKey.Name,
		Description:  apikey.String(dbKey.Description),
		CustomerName: customerName,
		Hash:         dbKey.Hash,
		Environment:  dbKey.Environment,
		ActorID:      dbKey.ActorID,
		CreatorID:    dbKey.CreatorID,
		ExpiresAt:    date.FormatTimeToPtr(dbKey.ExpiresAt),
		DeletedAt:    date.FormatTimeToPtr(dbKey.DeletedAt),
		CreatedAt:    date.FormatTime(dbKey.CreatedAt),
		Contact:      apikey.DBToContact(dbKey.Contact),
		Active:       dbKey.Active,
		Labels:       dbKey.Labels,
	}

	var err error
	if tykResponse != nil { // if the key was deleted, we don't have tyk data
		result.Policies, err = policies.GetPoliciesByIDs(ctx, h.tykClient, tykResponse.ApplyPolicies) // filters out quota policies
		if err != nil {
			return nil, err
		}

		result.Quota = tykResponse.QuotaMax
		result.QuotaRemaining = tykResponse.QuotaRemaining
		result.RateLimit = apikey.RateLimit{
			Rate: tykResponse.Rate,
			Per:  tykResponse.Per,
		}
	}

	if rateLimit, ok := dbKey.Metadata["rate_limit"]; ok {
		jsonData, _ := json.Marshal(rateLimit)
		var rateLimit apikey.RateLimit
		err := json.Unmarshal(jsonData, &rateLimit)
		if err != nil {
			return nil, err
		}
		result.RateLimit = apikey.RateLimit{
			Rate: rateLimit.Rate,
			Per:  rateLimit.Per,
		}
	}

	if dbKey.Description != nil {
		result.Description = *dbKey.Description
	}

	return &result, nil
}

func (h *Handler) getCustomerName(ctx context.Context, actorID string) (string, error) {
	// Get customer data
	_, span := goot.Span(ctx, "get_from_keycloak")
	keycloakGroups, err := h.customerService.GetCustomersPaginated(ctx, customer.ListParams{})
	if err != nil {
		goot.EndSpanWithError(span, err, "failed to call keycloak")
		log.Err(err).Msg("error while communicating with keycloak")
		return "", err
	}

	goot.EndSpan(span)
	return getCustomerNameByActorID(keycloakGroups, actorID), nil
}

func bindGetRequest(ctx *goskell.Context) (apikey.KeyID, error) {
	var request apikey.KeyID
	err := ctx.ShouldBindUri(&request)
	return request, err
}
