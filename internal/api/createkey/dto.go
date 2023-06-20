// Package createkey defines create a new API key handler.
package createkey

import (
	"errors"

	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/date"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/validation"
	"gitlab.ci.fdmg.org/ci-api/tyk-sdk-go"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
)

// input represents request body.
type input struct {
	ActorID      string     `json:"actor_id" binding:"required"` // The reference to the actor. It binds an API key to a user/customer.
	Policies     []string   `json:"policies" binding:"required"` // The access policies to give, leave empty for none.
	QuotaEndDate *date.Date `json:"quota_end_date"`              // Date on which the key quota will expire at 00.00 (optional).
	Quota        int64      `json:"quota" binding:"min=-1"`      // The amount of calls the API Key can make (optional).
	Description  string     `json:"description"`                 // Description for the key (optional).
}

// validate validates request body.
func (i *input) validate(ctx *goskell.Context, tykAPI *tyk.APIClient) error {
	if !validation.ValidatePolicies(ctx, tykAPI, i.Policies) {
		return errors.New("invalid policy")
	}
	if i.QuotaEndDate != nil {
		if !validation.ValidateEndDate(i.QuotaEndDate) {
			return errors.New("quota end date must be greater than today")
		}
	}
	return nil
}

// workflowInput represents the workflow's request body.
type workflowInput struct {
	ActorID      string     // The reference to the actor. It binds an API key to a user/customer.
	Policies     []string   // The access policies to give, leave empty for none.
	QuotaEndDate *date.Date // Date on which the key quota will expire at 00.00 (optional).
	Quota        int64      // The amount of calls the API Key can make (optional).
	Description  string     // Description for the key (optional).
}

// output represents response body.
type output struct {
	Key  string `json:"key"`  // The created API key which can be used to access APIs.
	Hash string `json:"hash"` // The key hash. This is a unique identifier to each API key.
}

// workflowOutput represents the workflow response body.
type workflowOutput struct {
	Key  string
	Hash string
}
