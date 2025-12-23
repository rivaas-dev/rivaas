package keys

import (
	"net/http"

	"github.com/mitchellh/mapstructure"
	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/db"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell/json/problem"
)

type AuthorizationInput struct {
	User    User    `mapstructure:"user"`
	Request Request `mapstructure:"request"`
}

type User struct {
	ID string `mapstructure:"id"`
}

type Request struct {
	Method string `mapstructure:"method"`
	Path   string `mapstructure:"path"`
	Key    Key    `mapstructure:"key"`
}

type Key struct {
	ActorID   string `mapstructure:"actor_id"`
	CreatorID string `mapstructure:"creator_id"`
}

func (h *Handler) getAuthorizationInput(ctx *goskell.Context, key *db.Key) (map[string]any, error) {
	var authzIn map[string]any

	input := AuthorizationInput{
		User: User{
			ID: ctx.GetHeader("X-Customer-ID"),
		},
		Request: Request{
			Method: ctx.Request.Method,
			Path:   ctx.FullPath(),
		},
	}

	if key != nil {
		input.Request.Key = Key{
			ActorID:   key.ActorID,
			CreatorID: key.CreatorID,
		}
	}

	err := mapstructure.Decode(input, &authzIn)
	if err != nil {
		return nil, err
	}

	return authzIn, nil
}

func (h *Handler) isAuthorized(ctx *goskell.Context, key *db.Key) bool {
	authorizationInput, err := h.getAuthorizationInput(ctx, key)
	if err != nil {
		if err != nil {
			log.Err(err).Msg("error on checking authorization")
			goskell.ProblemJSON(
				ctx,
				problem.Details{
					Status: http.StatusInternalServerError,
					Title:  http.StatusText(http.StatusInternalServerError),
				},
			)

			return false
		}
	}

	authorized, err := h.omaClient.IsAuthorized(ctx, "/admin/api/allow", authorizationInput)
	if err != nil {
		log.Err(err).Msg("error on checking authorization")
		goskell.ProblemJSON(
			ctx,
			problem.Details{
				Status: http.StatusInternalServerError,
				Title:  http.StatusText(http.StatusInternalServerError),
			},
		)

		return false
	}

	if !authorized {
		goskell.ProblemJSON(
			ctx,
			problem.Details{
				Status: http.StatusForbidden,
				Title:  http.StatusText(http.StatusForbidden),
			},
		)

		return false
	}

	return true
}
