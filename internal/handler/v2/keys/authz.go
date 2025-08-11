package keys

import (
	"errors"
	"fmt"
	"github.com/mitchellh/mapstructure"
	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/customers"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/handler/v2/keys/apikey"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/headers"
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
	ID           string       `mapstructure:"id"`
	Roles        []string     `mapstructure:"roles"`
	NumberOfKeys NumberOfKeys `mapstructure:"number_of_keys"`
}

type NumberOfKeys struct {
	Current int `mapstructure:"current"`
	Max     int `mapstructure:"max"`
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

	if key != nil {
		input.Key = *key
	}

	err = mapstructure.Decode(input, &authzIn)
	if err != nil {
		return nil, err
	}

	return authzIn, nil
}

func (h *Handler) getSubscriptionAuthorizationInput(ctx *goskell.Context, key *Key, subscription customers.Subscription, environment string) (map[string]any, error) {
	var authzIn map[string]any

	authHeaders, err := headers.GetAuthorization(ctx)
	if err != nil {
		return nil, err
	}

	var maxKeyCount, currentKeyCount int
	if subscription.APIKeys == nil {
		return nil, errors.New("no API keys limits found")
	}

	switch environment {
	case apikey.ProdEnv:
		maxKeyCount, currentKeyCount = subscription.APIKeys.Production.MaxCount, subscription.APIKeys.Production.CurrentCount
	case apikey.SandboxEnv:
		maxKeyCount, currentKeyCount = subscription.APIKeys.Sandbox.MaxCount, subscription.APIKeys.Sandbox.CurrentCount
	default:
		return nil, fmt.Errorf("invalid environment '%s'", environment)
	}

	input := AuthorizationInput{
		User: User{
			ID:    authHeaders.CustomerID,
			Roles: authHeaders.Roles,
			NumberOfKeys: NumberOfKeys{
				Current: currentKeyCount,
				Max:     maxKeyCount,
			},
		},
		Request: Request{
			Method: ctx.Request.Method,
			Path:   ctx.FullPath(),
		},
	}

	if key != nil {
		input.Key = *key
	}

	err = mapstructure.Decode(input, &authzIn)
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

	return h.isInputAuthorized(ctx, authorizationInput)
}

func (h *Handler) isSubscriptionAuthorized(ctx *goskell.Context, key *Key, subscription customers.Subscription, environment string) bool {
	authorizationInput, err := h.getSubscriptionAuthorizationInput(ctx, key, subscription, environment)
	if err != nil {
		log.Err(err).Msg("error on marshaling response")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return false
	}

	return h.isInputAuthorized(ctx, authorizationInput)
}

func (h *Handler) isInputAuthorized(ctx *goskell.Context, authorizationInput map[string]any) bool {
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
