// Package createkey defines create a new API key handler.
package createkey

import (
	"errors"

	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/date"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/validation"
	"gitlab.ci.fdmg.org/ci-api/tyk-sdk-go"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
)

type contact struct {
	Emails []string
	Users  []uint
}

// input represents request body.
type input struct {
	ActorID      string     `json:"actor_id" binding:"required"` // The reference to the actor. It binds an API key to a user/customer.
	Policies     []string   `json:"policies" binding:"required"` // The access policies to give, leave empty for none.
	QuotaEndDate *date.Date `json:"quota_end_date"`              // Date on which the key quota will expire at 00.00 (optional).
	Quota        int64      `json:"quota" binding:"min=-1"`      // The amount of calls the API Key can make (optional).
	Description  string     `json:"description"`                 // Description for the key (optional).
	Contact      contact    `json:"contacts,omitempty"`          // Contacts information.
}

// validate validates request body.
func (i *input) validate(ctx *goskell.Context, tykAPI *tyk.APIClient) error {
	// Validate policies.
	if !validation.ValidatePolicies(ctx, tykAPI, i.Policies) {
		return errors.New("invalid policy")
	}
	// Validate quota end date.
	if i.QuotaEndDate != nil {
		if !validation.ValidateEndDate(i.QuotaEndDate) {
			return errors.New("quota end date must be greater than today")
		}
	}
	// Validate contact emails.
	if len(i.Contact.Emails) > 0 {
		if !validation.ValidateEmail(i.Contact.Emails) {
			return errors.New("one or more contact emails are incorrect")
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
	Contact      contact    // Contacts information.
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
