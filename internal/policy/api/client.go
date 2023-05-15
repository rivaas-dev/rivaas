package api

import (
	"context"
	"net/http"

	"gitlab.ci.fdmg.org/ci-api/tyk-sdk-go"
)

//go:generate mockgen -destination=../mock_policies_client.go -package=policy -source=client.go ClientInterface

// ClientInterface to CRUD keys
type ClientInterface interface {
	ListPolicies(ctx context.Context) ([]tyk.Policy, *http.Response, error)
}
