// Package tyk handles Tyk Gateway webhook events.
package tyk

import (
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/db"
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/gomes/publisher"
)

// Handler handles Tyk Gateway webhook events.
type Handler struct {
	keysRepository db.DatabaseExecer
	publisher      publisher.Publisher
	webhookSecret  string
	publishEvents  bool
	emailRecipient string
}

// New constructs a new Handler.
func New(keysRepository db.DatabaseExecer, pub publisher.Publisher, webhookSecret string, publishEvents bool, emailRecipient string) *Handler {
	return &Handler{
		keysRepository: keysRepository,
		publisher:      pub,
		webhookSecret:  webhookSecret,
		publishEvents:  publishEvents,
		emailRecipient: emailRecipient,
	}
}
