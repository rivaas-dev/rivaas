package message

import (
	"fmt"
	"time"

	"github.com/companyinfo/gourn"
	"github.com/oklog/ulid/v2"
	"gitlab.ci.fdmg.org/ci-api/cigourn/urn"
	gomessage "gitlab.ci.fdmg.org/ci-api/go-pkgs/gomes/message"
)

const (
	// quotaExceededAction is the action for quota exceeded events.
	quotaExceededAction = "quota-exceeded"
)

// Service URN constants for WHERE field.
const (
	projectDomain = "admin"
	projectType   = "api"
	projectName   = "admin-api"
)

// QuotaEventData contains the metadata for a quota-related event.
type QuotaEventData struct {
	KeyID          string `json:"keyId"`
	TriggerLimit   string `json:"triggerLimit"`
	EmailRecipient string `json:"emailRecipient,omitempty"`
}

// NewQuotaExceededMessage creates a new message for quota exceeded events.
// The `what` parameter should be the API key URN (starting with urn:api:key:).
func NewQuotaExceededMessage(what *gourn.URN, data QuotaEventData) gomessage.Message {
	meta := map[string]any{
		"apiKeyID":     data.KeyID,
		"triggerLimit": data.TriggerLimit,
		// NOTE: This is a workaround solution. The apiKeyDescription field contains
		// a hardcoded message template. Once a dedicated email template is created
		// for quota exceeded notifications, this field should be removed and the
		// template should handle the message formatting.
		"apiKeyDescription": fmt.Sprintf(
			"Your API key quota has exceeded the %s%% threshold. Please contact support if you need to increase your quota.",
			data.TriggerLimit,
		),
	}

	if data.EmailRecipient != "" {
		meta["emailRecipient"] = data.EmailRecipient
	}

	return gomessage.Message{
		ID:       ulid.Make().String(),
		Who:      what,
		Did:      quotaExceededAction,
		What:     what,
		Where:    NewWhereURN(),
		When:     gomessage.Time(time.Now()),
		Metadata: meta,
	}
}

// NewWhereURN creates the service origin URN for admin-api.
func NewWhereURN() *gourn.URN {
	return urn.NewURN(projectDomain, projectType, projectName)
}
