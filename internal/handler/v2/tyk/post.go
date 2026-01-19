package tyk

import (
	"context"
	"net/http"

	"github.com/companyinfo/gourn"
	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/message"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/tyk"
	gomessage "gitlab.ci.fdmg.org/ci-api/go-pkgs/gomes/message"
	"gitlab.ci.fdmg.org/datacluster/golibs/goot"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"go.opentelemetry.io/otel/attribute"
)

const (
	// HeaderTykMonitorSecret is the header name for Tyk monitor webhook secret.
	HeaderTykMonitorSecret = "x-tyk-monitor-secret" // #nosec G101 -- Header name, not a credential
)

// POST handles incoming Tyk Gateway webhook events.
func (h *Handler) POST(ctx *goskell.Context) {
	// Validate webhook secret
	secret := ctx.GetHeader(HeaderTykMonitorSecret)
	if secret != h.webhookSecret {
		log.Warn().
			Str("receivedSecret", secret).
			Msg("invalid or missing Tyk monitor secret")
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	// Parse request body
	var event EventInput
	if err := ctx.ShouldBindJSON(&event); err != nil {
		log.Error().Err(err).Msg("failed to parse Tyk event payload")
		goskell.JsonAPIError(ctx, "invalid event payload", err, http.StatusBadRequest)
		return
	}

	// Log the event (without sensitive data like the raw key)
	log.Info().
		Str("event", string(event.Event)).
		Str("message", event.Message).
		Str("org", event.Org).
		Str("triggerLimit", event.TriggerLimit).
		Str("path", event.Path).
		Str("origin", event.Origin).
		Bool("hasKey", event.Key != "").
		Msg("received Tyk event")

	// Route to appropriate handler based on event type
	switch {
	case event.IsQuotaEvent():
		h.handleQuotaEvent(ctx, &event)
	// Dispatch other events to the appropriate handler for future implementation
	default:
		log.Warn().
			Str("event", string(event.Event)).
			Msg("unhandled Tyk event type")
		// Return 204 No Content for unhandled events to acknowledge receipt without action.
		// Using 2xx prevents Tyk from retrying events we intentionally don't handle.
		ctx.Status(http.StatusNoContent)
		return
	}

	ctx.Status(http.StatusAccepted)
}

// handleQuotaEvent processes quota-related events.
func (h *Handler) handleQuotaEvent(ctx *goskell.Context, event *EventInput) {
	log.Info().
		Str("event", string(event.Event)).
		Str("triggerLimit", event.TriggerLimit).
		Str("org", event.Org).
		Msg("processing quota event")

	// Skip if publishing is disabled
	if !h.publishEvents {
		log.Debug().Msg("event publishing is disabled, skipping")
		return
	}

	// Skip if no key is provided (org-level events without key)
	if event.Key == "" {
		log.Debug().Msg("no API key in event, skipping message publishing")
		return
	}

	// Look up key information from database using the key hash
	keyHash := tyk.HashKey(event.Key)
	_, span := goot.Span(ctx.Request.Context(), "get_key_by_hash",
		attribute.String("key_hash", keyHash),
	)
	key, err := h.keysRepository.GetKeyByHash(keyHash)
	if err != nil {
		goot.EndSpanWithError(span, err, "failed to look up API key")
		log.Error().
			Err(err).
			Str("keyHash", keyHash).
			Msg("failed to look up API key")
		return
	}

	if key == nil {
		goot.EndSpan(span)
		log.Warn().
			Str("keyHash", keyHash).
			Msg("API key not found in database")
		return
	}
	goot.EndSpan(span)

	// Parse the ActorID into a URN for the WHAT field
	whatURN, err := gourn.Parse(key.ActorID)
	if err != nil {
		log.Error().
			Err(err).
			Str("actorID", key.ActorID).
			Msg("failed to parse ActorID as URN")
		return
	}

	// Create the message data
	msgData := message.QuotaEventData{
		KeyID:          key.ID,
		TriggerLimit:   event.TriggerLimit,
		EmailRecipient: h.emailRecipient,
	}

	// Create the quota exceeded message
	msg := message.NewQuotaExceededMessage(whatURN, msgData)

	// Publish the message via EventBridge
	if err := h.publishEvent(ctx.Request.Context(), msg); err != nil {
		log.Error().
			Err(err).
			Str("messageID", msg.ID).
			Msg("failed to publish quota event message")
		return
	}

	log.Info().
		Str("messageID", msg.ID).
		Str("keyID", key.ID).
		Str("event", string(event.Event)).
		Msg("quota event message published successfully")
}

// publishEvent publishes a message to EventBridge using the gomes publisher.
func (h *Handler) publishEvent(ctx context.Context, msg gomessage.Message) error {
	_, span := goot.Span(ctx, "publish_to_eventbridge",
		attribute.String("message_id", msg.ID),
	)

	if err := h.publisher.Publish(ctx, msg); err != nil {
		goot.EndSpanWithError(span, err, "failed to publish to EventBridge")
		return err
	}
	goot.EndSpan(span)

	log.Debug().
		Str("messageID", msg.ID).
		Msg("message published to EventBridge")

	return nil
}
