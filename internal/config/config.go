package config

import (
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/customers"
	gomesconfig "gitlab.ci.fdmg.org/ci-api/go-pkgs/gomes/config"
	"go.companyinfo.dev/keycloak"
)

// Config represents application configuration.
type Config struct {
	Host           string
	Port           uint16
	LogLevel       string
	KeyCloak       KeyCloakConfig
	Database       Database
	Tyk            Tyk
	Temporal       Temporal
	OMA            OMA
	OpenTelemetry  OpenTelemetry
	Solvimon       Solvimon
	Pagination     Pagination
	PricingPlans   map[string]customers.PricingPlan
	APIKeyDefaults APIKeyDefaults
	Publisher      Publisher
}

// Publisher represents EventBridge publisher configuration.
type Publisher struct {
	AWS            gomesconfig.Config // AWS configuration for EventBridge
	EventBusName   string             // EventBridge event bus name
	Enabled        bool               // Whether publishing is enabled
	EmailRecipient string             // Alert recipient email (e.g., support-ws, PM)
}

// Database represents Database configuration.
type Database struct {
	Host     string
	Port     uint16
	Username string
	Password string
	Name     string
	SSLMode  string
}

// Tyk represents Tyk configuration.
type Tyk struct {
	URL           string
	Secret        string
	Debug         bool
	WebhookSecret string // Secret used to validate incoming Tyk monitor webhook requests
}

// Temporal represents Temporal configuration.
type Temporal struct {
	HostPort  string
	Namespace string
}

// OMA represents OMA configuration.
type OMA struct {
	Address   string
	Namespace string
	OPA       OPA
}

// OPA represents OPA configuration.
type OPA struct {
	Address string
}

type KeyCloakConfig struct {
	Credentials        keycloak.Config
	BrifRepresentation bool
	First              int
	Max                int
}

type OpenTelemetry struct {
	URL string
}

// Solvimon is the config for the solvimon client
type Solvimon struct {
	BaseUrl string
	ApiKey  string
}

type Pagination struct {
	MaxPageSize     uint
	DefaultPageSize uint
}

type APIKeyDefaults struct {
	Policies []string
}
