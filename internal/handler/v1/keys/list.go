// Package keys defines all methods of the API key.
package keys

import (
	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/date"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/db"
	"gitlab.ci.fdmg.org/datacluster/golibs/goot"
	"net/http"
	"time"

	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell/json/problem"
)

// ListInput represents the request body of the LIST endpoint.
type ListInput struct {
	ActorID     string `form:"actor_id"`    // ActorID full urn of the customer. Example: urn:api:key:19fd5d47-91ea-4cac-a8e0-ea9e295fa44b:f82dddf1-4f06-4fa0-80bd-c2014d5f9540:01HX6GV6CV8NPR1XQRRQQZTMXN
	Description string `form:"description"` // Description of the api key. Usually is the customer name and type of key, like Prod key customer X
	CustomerID  string `form:"customer_id"` // CustomerID is the ID of the overall Customer. Example: 19fd5d47-91ea-4cac-a8e0-ea9e295fa44b
	AccountID   string `form:"account_id"`  // AccountID is the ID of the account. An account can be for example API. Example: f82dddf1-4f06-4fa0-80bd-c2014d5f9540
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
	_, span := goot.Span(ctx.Request.Context(), "get_from_database")
	keys, err := h.keysRepository.GetKeys(request.ActorID, request.Description, request.CustomerID, request.AccountID)
	if err != nil {
		goot.EndSpanWithError(span, err, "failed to call database")
		log.Err(err).Msg("error while communicating with DB")
		goskell.ProblemJSON(ctx, problem.Details{Status: http.StatusInternalServerError})
		return
	}
	goot.EndSpan(span)

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
