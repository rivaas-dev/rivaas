package customers

import (
	"errors"
	"github.com/mitchellh/mapstructure"
	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/headers"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"net/http"
)

type AuthorizationInput struct {
	User     User     `mapstructure:"user"`
	Request  Request  `mapstructure:"request"`
	Customer Customer `mapstructure:"customer"`
}

type User struct {
	ID    string   `mapstructure:"id"`
	Roles []string `mapstructure:"roles"`
}

type Request struct {
	Method string `mapstructure:"method"`
	Path   string `mapstructure:"path"`
}

type Customer struct {
	ID string `mapstructure:"id"`
}

func (h *Handler) getAuthorizationInput(ctx *goskell.Context, customer *Customer) (map[string]any, error) {
	var authzIn map[string]any

	authHeaders, err := headers.GetAuthorization(ctx)
	if err != nil {
		return nil, err
	}

	input := AuthorizationInput{
		User: User{
			ID:    authHeaders.CustomerID,
			Roles: authHeaders.Roles,
		},
		Request: Request{
			Method: ctx.Request.Method,
			Path:   ctx.FullPath(),
		},
	}

	if customer != nil {
		input.Customer = *customer
	}

	err = mapstructure.Decode(input, &authzIn)
	if err != nil {
		return nil, err
	}

	return authzIn, nil
}

func (h *Handler) isAuthorized(ctx *goskell.Context, customer *Customer) bool {
	authorizationInput, err := h.getAuthorizationInput(ctx, customer)
	if err != nil {
		log.Err(err).Msg("error on marshaling response")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return false
	}

	authorized, err := h.omaClient.IsAuthorized(ctx, "/admin/api/allow", authorizationInput)
	if err != nil {
		log.Err(err).Msg("error on checking authorization")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return false
	}

	if !authorized {
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusForbidden), errors.New("forbidden"), http.StatusForbidden)
		return false
	}

	return true
}
