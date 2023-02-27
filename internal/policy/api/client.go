package api

import (
	"context"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-sdk-go"
	"net/http"
)

//go:generate mockgen -destination=../mock_policies_client.go -package=policy -source=client.go ClientInterface

// ClientInterface to CRUD keys
type ClientInterface interface {
	ListPolicies(ctx context.Context) ([]tyk.Policy, *http.Response, error)
}
