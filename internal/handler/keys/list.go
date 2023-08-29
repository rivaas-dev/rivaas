// Package keys defines all methods of the API key.
package keys

import (
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/date"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/db"
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
	ID          string            `json:"id"`
	CreatorID   string            `json:"creator_id"`
	ActorID     string            `json:"actor_id"`
	ExpiresAt   *date.Date        `json:"expires_at"`
	Description *string           `json:"description"`
	CreationAt  time.Time         `json:"creation_date"`
	Contact     Contact           `json:"contacts"`
	Active      bool              `json:"active"`
	RateLimit   RateLimit         `json:"rate_limit"`
	Environment ApikeyEnvironment `json:"environment"`
	Labels      map[string]string `json:"labels"`
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

	// Prepare the response.
	response := h.convertListDBResultToJSON(keys)
	ctx.JSON(http.StatusCreated, response)
}

func (h *Handler) convertListDBResultToJSON(keys []*db.Key) []ListOutput {
	var response []ListOutput
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

		response = append(response, ListOutput{
			ID:          key.ID,
			CreatorID:   key.CreatorID,
			ActorID:     key.ActorID,
			ExpiresAt:   key.ExpiresAt,
			Description: key.Description,
			CreationAt:  key.CreatedAt,
			Contact:     Contact(key.Contact),
			Active:      key.Active,
			RateLimit:   rl,
			Environment: key.Environment,
			Labels:      key.Labels,
		})
	}
	return response
}
