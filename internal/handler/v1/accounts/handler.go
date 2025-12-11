package accounts

import (
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/customer"
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/solvimon"
)

// Handler handles keys requests
type Handler struct {
	customerService customer.CustomerClient
	solvimonClient  *solvimon.Client
}

// New constructs a new Handler.
func New(customerService customer.CustomerClient, SolvimonClient *solvimon.Client) *Handler {
	return &Handler{
		customerService: customerService,
		solvimonClient:  SolvimonClient,
	}
}
