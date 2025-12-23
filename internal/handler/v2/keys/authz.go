package keys

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/customers"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/handler/v2/authz"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/handler/v2/keys/apikey"
	"gitlab.ci.fdmg.org/ci-api/cigourn/api"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
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
	Key    *Key   `mapstructure:"key,omitempty"`
}

type Key struct {
	ActorID     string    `mapstructure:"actor_id"`
	CreatorID   string    `mapstructure:"creator_id"`
	Environment string    `mapstructure:"environment"`
	Policies    *[]string `mapstructure:"policies"`
	Active      *bool     `mapstructure:"active"`
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

func (k *Key) WithPolicies(policies *[]string) *Key {
	k.Policies = policies
	return k
}

func (k *Key) WithActive(active *bool) *Key {
	k.Active = active
	return k
}

func (k *Key) WithEnvironment(environment string) *Key {
	k.Environment = environment
	return k
}

func NewKeyCustomerAccountID(customerID, accountID, creatorID string) *Key {
	key := api.Key{CustomerID: customerID, AccountID: accountID}
	return &Key{
		ActorID:   key.String(),
		CreatorID: creatorID,
	}
}

func (h *Handler) getAuthorizationInput(ctx *goskell.Context, key *Key) (map[string]any, error) {
	return authz.BuildInput(ctx, func(user authz.BaseUser, req authz.BaseRequest) (any, error) {
		in := AuthorizationInput{
			User: User{ID: user.ID, Roles: user.Roles},
			Request: Request{Method: req.Method, Path: req.Path, Key: key},
		}
		if key != nil {
			in.Key = *key
		}
		return in, nil
	})
}

func (h *Handler) getSubscriptionAuthorizationInput(ctx *goskell.Context, key *Key, subscription customers.Subscription) (map[string]any, error) {
	if key == nil {
		return nil, errors.New("key is nil, required to validate subscription")
	}

	var maxKeyCount, currentKeyCount int
	if subscription.APIKeys == nil {
		return nil, errors.New("no API keys limits found")
	}

	switch key.Environment {
	case apikey.ProdEnv:
		maxKeyCount, currentKeyCount = subscription.APIKeys.Production.MaxCount, subscription.APIKeys.Production.CurrentCount
	case apikey.SandboxEnv:
		maxKeyCount, currentKeyCount = subscription.APIKeys.Sandbox.MaxCount, subscription.APIKeys.Sandbox.CurrentCount
	default:
		return nil, fmt.Errorf("invalid environment '%s'", key.Environment)
	}

	return authz.BuildInput(ctx, func(user authz.BaseUser, req authz.BaseRequest) (any, error) {
		in := AuthorizationInput{
			User: User{
				ID:    user.ID,
				Roles: user.Roles,
				NumberOfKeys: NumberOfKeys{Current: currentKeyCount, Max: maxKeyCount},
			},
			Request: Request{Method: req.Method, Path: req.Path},
		}
		return in, nil
	})
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

func (h *Handler) isSubscriptionAuthorized(ctx *goskell.Context, key *Key, subscription customers.Subscription) bool {
	authorizationInput, err := h.getSubscriptionAuthorizationInput(ctx, key, subscription)
	if err != nil {
		log.Err(err).Msg("error on marshaling response")
		goskell.JsonAPIError(ctx, http.StatusText(http.StatusInternalServerError), err, http.StatusInternalServerError)
		return false
	}

	return h.isInputAuthorized(ctx, authorizationInput)
}

func (h *Handler) isInputAuthorized(ctx *goskell.Context, authorizationInput map[string]any) bool {
	return authz.Check(ctx, h.omaClient, authorizationInput)
}
