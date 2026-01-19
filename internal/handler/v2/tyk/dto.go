package tyk

// EventType represents the type of Tyk event.
type EventType string

// Tyk event types.
const (
	// EventTriggerExceeded is the event type for quota trigger threshold reached (e.g., 80%).
	EventTriggerExceeded EventType = "TriggerExceeded"
	// EventQuotaExceeded is the event type for quota fully exhausted (100%).
	EventQuotaExceeded EventType = "QuotaExceeded"
	// EventOrgQuotaExceeded is the event type for organization quota exceeded.
	EventOrgQuotaExceeded EventType = "OrgQuotaExceeded"

	// EventRateLimitExceeded is the event type for rate limit exceeded.
	EventRateLimitExceeded EventType = "RatelimitExceeded"
	// EventOrgRateLimitExceeded is the event type for organization rate limit exceeded.
	EventOrgRateLimitExceeded EventType = "OrgRateLimitExceeded"
)

// EventInput represents the incoming Tyk webhook event payload.
type EventInput struct {
	Event        EventType `json:"event"`         // Type of the event (e.g., "QuotaExceeded", "OrgQuotaExceeded")
	Message      string    `json:"message"`       // Human-readable message from Tyk
	Org          string    `json:"org"`           // Organization ID
	Key          string    `json:"key"`           // Raw API key (empty for org-level events)
	TriggerLimit string    `json:"trigger_limit"` // Percentage threshold that triggered the event (e.g., "80")
	Path         string    `json:"path"`          // API path that triggered the event (optional)
	Origin       string    `json:"origin"`        // Origin IP address (optional)
}

// IsQuotaEvent returns true if the event is related to quota.
func (e *EventInput) IsQuotaEvent() bool {
	switch e.Event {
	case EventTriggerExceeded, EventQuotaExceeded, EventOrgQuotaExceeded:
		return true
	default:
		return false
	}
}

// IsRateLimitEvent returns true if the event is related to rate limiting.
func (e *EventInput) IsRateLimitEvent() bool {
	switch e.Event {
	case EventRateLimitExceeded, EventOrgRateLimitExceeded:
		return true
	default:
		return false
	}
}
