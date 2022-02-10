package api

import (
	"context"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-sdk-go"
	"net/http"
)

//go:generate mockgen -destination=../mock_keys_client.go -package=key -source=client.go ClientInterface

//ClientInterface to CRUD keys
type ClientInterface interface {
	AddKey(ctx context.Context, localVarOptionals *tyk.AddKeyOpts) (tyk.ApiModifyKeySuccess, *http.Response, error)
	DeleteKey(ctx context.Context, keyID string) (tyk.ApiStatusMessage, *http.Response, error)
}
