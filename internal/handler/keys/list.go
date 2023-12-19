// Package keys defines all methods of the API key.
package keys

import (
	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/date"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/db"
	"net/http"
	"time"

	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell/json/problem"
)

// ListInput represents the request body of the LIST endpoint.
type ListInput struct {
	ActorID     string `form:"actor_id"`
	Description string `form:"description"`
}

// ListOutput represents the list of key's information.
type ListOutput struct {
	ID           string            `json:"id"`
	CustomerName string            `json:"customer_name"`
	Hash         string            `json:"hash"`
	CreatorID    string            `json:"creator_id"`
	ActorID      string            `json:"actor_id"`
	ExpiresAt    *date.Date        `json:"expires_at"`
	Description  *string           `json:"description"`
	CreationAt   time.Time         `json:"creation_date"`
	Contact      Contact           `json:"contacts"`
	Active       bool              `json:"active"`
	RateLimit    RateLimit         `json:"rate_limit"`
	Environment  ApikeyEnvironment `json:"environment"`
	Labels       map[string]string `json:"labels"`
}

// LIST handles GET requests on the endpoint to get list of keys.
func (h *Handler) LIST(ctx *goskell.Context) {
	// Parse request body.
	var request ListInput
	if err := ctx.ShouldBind(&request); err != nil {
		goskell.ProblemJSON(
			ctx,
			problem.Details{
				Title:  http.StatusText(http.StatusBadRequest),
				Status: http.StatusBadRequest,
			},
		)
		return
	}

	// Fetch keys from the database.
	keys, err := h.keysRepository.GetKeys(request.ActorID, request.Description)
	if err != nil {
		log.Err(err).Msg("error while communicating with DB")
		goskell.ProblemJSON(ctx, problem.Details{Status: http.StatusInternalServerError})
		return
	}

	if !h.isAuthorized(ctx, nil) {
		// The appropriate response is already handled in "isAuthorized()"
		return
	}

	// Prepare the response.
	response := h.convertListDBResultToJSON(ctx, keys)
	ctx.JSON(http.StatusOK, response)
}

func (h *Handler) convertListDBResultToJSON(ctx *goskell.Context, keys []*db.Key) []ListOutput {
	response := make([]ListOutput, 0)
	for _, key := range keys {
		rl := RateLimit{
			Rate: 0,
			Per:  0,
		}

		if key.Metadata != nil {
			if data, ok := key.Metadata["rate_limit"]; ok {
				castedData := data.(map[string]any)
				rl.Rate = uint(castedData["Rate"].(float64))
				rl.Per = uint(castedData["Per"].(float64))
			}
		}

		// call keycloak to get the customer name, set it as unknown to avoid panic when not found
		keycloakGroups, err := h.keycloakClient.GetGroups(ctx, h.keycloakConfig.BrifRepresentation, h.keycloakConfig.First, h.keycloakConfig.Max)
		if err != nil {
			log.Err(err).Msg("cant find client in keycloak")
		}
		// Define the response
		response = append(response, ListOutput{
			ID:           key.ID,
			CustomerName: internal.GetCustomerName(keycloakGroups, *key),
			CreatorID:    key.CreatorID,
			Hash:         key.Hash,
			ActorID:      key.ActorID,
			ExpiresAt:    key.ExpiresAt,
			Description:  key.Description,
			CreationAt:   key.CreatedAt,
			Contact:      Contact(key.Contact),
			Active:       key.Active,
			RateLimit:    rl,
			Environment:  key.Environment,
			Labels:       key.Labels,
		})
	}
	return response
}
