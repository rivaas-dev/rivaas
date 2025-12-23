package authz

import (
	"errors"
	"net/http"

	"github.com/mitchellh/mapstructure"
	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/headers"
	oma "gitlab.ci.fdmg.org/ci-api/oma/pkg/client"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
)

// BaseUser defines common user fields used in authorization input payloads.
type BaseUser struct {
	ID    string   `mapstructure:"id,omitempty"`
	Roles []string `mapstructure:"roles"`
}

// BaseRequest defines common request fields used in authorization input payloads.
type BaseRequest struct {
	Method string `mapstructure:"method"`
	Path   string `mapstructure:"path"`
}

// BuildBase constructs common user and request structures from the context.
func BuildBase(ctx *goskell.Context) (BaseUser, BaseRequest, error) {
	authHeaders, err := headers.GetAuthorization(ctx)
	if err != nil {
		return BaseUser{}, BaseRequest{}, err
	}

	user := BaseUser{
		ID:    authHeaders.CustomerID,
		Roles: authHeaders.Roles,
	}
	request := BaseRequest{
		Method: ctx.Request.Method,
		Path:   ctx.FullPath(),
	}

 return user, request, nil
}

// BuildInput builds an authorization input map by combining the common base
// (user, request) with a package-specific payload constructed by the builder.
// It converts the composed struct to map[string]any using mapstructure.
func BuildInput(ctx *goskell.Context, builder func(BaseUser, BaseRequest) (any, error)) (map[string]any, error) {
	user, req, err := BuildBase(ctx)
	if err != nil {
		return nil, err
	}
	payload, err := builder(user, req)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := mapstructure.Decode(payload, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Check performs the authorization check against OMA and writes appropriate JSON API error responses.
// Returns true if authorized, false otherwise.
func Check(ctx *goskell.Context, omaClient *oma.Client, authorizationInput map[string]any) bool {
	authorized, err := omaClient.IsAuthorized(ctx, "/admin/api/allow", authorizationInput)
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
