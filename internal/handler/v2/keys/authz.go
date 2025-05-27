package keys

import (
	"errors"
	"github.com/mitchellh/mapstructure"
	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/cigourn/api"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"net/http"
)

type AuthorizationInput struct {
	User    User    `mapstructure:"user"`
	Request Request `mapstructure:"request"`
	Key     Key     `mapstructure:"key"`
}

type User struct {
	ID string `mapstructure:"id"`
}

type Request struct {
	Method string `mapstructure:"method"`
	Path   string `mapstructure:"path"`
}

type Key struct {
	ActorID   string `mapstructure:"actor_id"`
	CreatorID string `mapstructure:"creator_id"`
}

func NewKey(actorID, customerID, accountID, creatorID string) *Key {
	if actorID == "" {
		return NewKeyCustomerAccountID(customerID, accountID, creatorID)
	}

	return NewKeyActorID(actorID, creatorID)
}

func NewKeyActorID(actorID, creatorID string) *Key {
	return &Key{
		ActorID:   actorID,
		CreatorID: creatorID,
	}
}

func NewKeyCustomerAccountID(customerID, accountID, creatorID string) *Key {
	key := api.Key{CustomerID: customerID, AccountID: accountID}
	return &Key{
		ActorID:   key.String(),
		CreatorID: creatorID,
	}
}

func (h *Handler) getAuthorizationInput(ctx *goskell.Context, key *Key) (map[string]any, error) {
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
		input.Key = *key
	}

	err := mapstructure.Decode(input, &authzIn)
	if err != nil {
		return nil, err
	}

	return authzIn, nil
}

func (h *Handler) isAuthorized(ctx *goskell.Context, key *Key) bool {
	authorizationInput, err := h.getAuthorizationInput(ctx, key)
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
