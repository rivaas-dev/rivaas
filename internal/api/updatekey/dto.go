// Package updatekey updates a key.
package updatekey

import (
	"errors"
	"time"

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
	Hash         string     `uri:"id" binding:"required"` // Key ID.
	Policies     *[]string  `json:"policies"`             // The access policies to give, leave empty for none.
	QuotaEndDate *date.Date `json:"quota_end_date"`       // Date on which the key quota will expire at 00.00 (optional).
	Quota        *int64     `json:"quota"`                // The amount of calls the API Key can make (optional).
	Description  *string    `json:"description"`          // Description for the key (optional).
	Contact      *contact   `json:"contacts,omitempty"`   // Contacts information.
}

// validate validates request body.
func (i *input) validate(ctx *goskell.Context, tykAPI *tyk.APIClient) error {
	// Validate quota.
	if i.Quota != nil && *i.Quota < -1 {
		return errors.New("quota must be greater than equal -1")
	}
	// Validate policies.
	if i.Policies != nil {
		if !validation.ValidatePolicies(ctx, tykAPI, *i.Policies) {
			return errors.New("invalid policy")
		}
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

// workflowInput represents the workflow request body.
type workflowInput struct {
	Hash         string     // Key ID.
	Policies     *[]string  // The access policies to give, leave empty for none.
	QuotaEndDate *date.Date // Date on which the key quota will expire at 00.00 (optional).
	Quota        *int64     // The amount of calls the API Key can make (optional).
	Description  *string    // Description for the key (optional).
	Contact      *contact   // Contacts information.
}

// output represents response body.
type output struct {
	ActorID      string     `json:"actor_id"`
	Policies     []string   `json:"policies"`
	QuotaEndDate *date.Date `json:"quota_end_date"`
	Quota        int64      `json:"quota"`
	Description  string     `json:"description"`
	CreatedDate  time.Time  `json:"created_date"`
	Contact      contact    `json:"contacts"`
}

// workflowOutput represents the workflow response body.
type workflowOutput struct {
	ActorID      string
	Policies     []string
	QuotaEndDate *date.Date
	Quota        int64
	Description  string
	CreatedAt    time.Time
	Contact      contact
}
