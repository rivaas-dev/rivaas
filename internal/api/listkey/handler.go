// Package listkey returns list of keys.
package listkey

import (
	"net/http"

	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/db"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell/json/problem"
)

// Handler handles keys requests
type Handler struct {
	keysRepository db.DatabaseExecer
}

// New constructs a new Handler.
func New(keysRepository db.DatabaseExecer) *Handler {
	return &Handler{
		keysRepository: keysRepository,
	}
}

// Handle handles endpoint requests.
func (h *Handler) Handle(ctx *goskell.Context) {
	// Parse request body.
	var request input
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
	response := h.convertDBResultToJSON(keys)
	ctx.JSON(http.StatusCreated, response)
}

func (h *Handler) convertDBResultToJSON(keys []*db.Key) []output {
	var response []output = make([]output, len(keys))
	for _, key := range keys {
		response = append(response, output{
			Hash:         key.Hash,
			ActorID:      key.ActorID,
			QuotaEndDate: key.QuotaEndDate,
			Description:  key.Description,
			CreationAt:   key.CreatedAt,
			Contact:      contact(key.Contact),
		})
	}
	return response
}
