package policies

import (
	"net/http"

	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/handler/v2/authz"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
)

type AuthorizationInput struct {
	User    User    `mapstructure:"user"`
	Request Request `mapstructure:"request"`
}

type User struct {
	Roles []string `mapstructure:"roles"`
}

type Request struct {
	Method string `mapstructure:"method"`
	Path   string `mapstructure:"path"`
}

func (h *Handler) getAuthorizationInput(ctx *goskell.Context) (map[string]any, error) {
	return authz.BuildInput(ctx, func(user authz.BaseUser, req authz.BaseRequest) (any, error) {
		in := AuthorizationInput{
			User: User{Roles: user.Roles},
			Request: Request{Method: req.Method, Path: req.Path},
		}
		return in, nil
	})
}

func (h *Handler) isAuthorized(ctx *goskell.Context) bool {
	authorizationInput, err := h.getAuthorizationInput(ctx)
	if err != nil {
		log.Err(err).Msg("error on marshaling response")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return false
	}

	return authz.Check(ctx, h.omaClient, authorizationInput)
}
