package main

import (
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/config"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/db"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/handler/accounts"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/handler/keys"
	policiesHandler "gitlab.ci.fdmg.org/ci-api/admin-api/internal/handler/policies"
	"gitlab.ci.fdmg.org/ci-api/admin-api/policies"
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/keycloak"
	oma "gitlab.ci.fdmg.org/ci-api/oma/pkg/client"
	"gitlab.ci.fdmg.org/ci-api/tyk-sdk-go"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"gitlab.ci.fdmg.org/datacluster/nl/webservices/goconfig"
	"go.temporal.io/sdk/client"
	"net/http"
	"net/url"
)

// App defines app configuration.
type App struct {
	config.Config `mapstructure:"app" consul:"admin-api" codec:"yaml"`
}

func main() {
	if err := run(context.Background()); err != nil {
		log.Err(err).Msg("error while starting the application")
	}
}

func run(ctx context.Context) error {
	// Load config.
	var cfg App
	err := goconfig.Unmarshal(&cfg)
	if err != nil {
		return err
	}

	// Initialize Goskell server.
	server := goskell.NewServer()
	server.WithMetrics("admin_api")

	// Connect to the database.
	keysRepository, err := db.New(
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Username,
		cfg.Database.Password,
		cfg.Database.Name,
	)
	if err != nil {
		log.Err(err).Msg("could not connect to database")
		return err
	}

	// Connect to Tyk client.
	tykClient := newTykClient(cfg.Tyk)

	// Connect to Temporal client.
	temporalClient, err := client.Dial(
		client.Options{
			HostPort:  cfg.Temporal.HostPort,
			Namespace: cfg.Temporal.Namespace,
		},
	)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to dial Temporal server")
	}
	defer temporalClient.Close()

	// These configs are for invoking the interface and used in the handler
	keyCloakConfig := config.KeyCloakConfig{
		BrifRepresentation: cfg.KeyCloak.BrifRepresentation,
		First:              cfg.KeyCloak.First,
		Max:                cfg.KeyCloak.Max,
	}
	// Connect to Keycloak client
	keyCloakClient, err := keycloak.New(ctx, cfg.KeyCloak.Credentials)
	if err != nil {
		log.Fatal().
			Err(err).
			Msg("Unable to initialize keyCloak client")
	}

	// Connect to OMA and OPA
	omaClient := newOMAClient(ctx, cfg.OMA)

	// Register handlers.
	registerHandlers(server, keysRepository, tykClient, temporalClient, omaClient, keyCloakClient, keyCloakConfig)

	// Run server.
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	return server.Run(addr)
}

func newTykClient(cfg config.Tyk) *tyk.APIClient {
	// Prepare Tyk info.
	parsedServer, _ := url.Parse(cfg.URL)
	endpoint := fmt.Sprintf("%s%s", parsedServer.Host, parsedServer.Path)
	return tyk.NewAPIClient(
		&tyk.Configuration{
			Host:          endpoint,
			Scheme:        parsedServer.Scheme,
			Debug:         cfg.Debug,
			DefaultHeader: map[string]string{"x-tyk-authorization": cfg.Secret},
		},
	)
}

func newOMAClient(ctx context.Context, cfg config.OMA) *oma.Client {
	baseOMAURL, err := url.Parse(cfg.Address)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to parse OMA URL")
	}

	baseOPAURL, err := url.Parse(cfg.OPA.Address)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to parse OPA URL")
	}

	c, err := oma.New(baseOMAURL, baseOPAURL, oma.WithNamespace(cfg.Namespace), oma.WithHTTPClient(&http.Client{}))
	if err != nil {
		log.Fatal().Err(err).Msg("unable to initialize OMA client")
	}

	err = policies.InitializeRegoFiles(ctx, c)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to initialize Rego files on OMA")
	}

	return c
}

func registerHandlers(
	server *goskell.Server,
	dbClient db.DatabaseExecer,
	tykClient *tyk.APIClient,
	temporalClient client.Client,
	omaClient *oma.Client,
	keyCloakClient keycloak.Client,
	keyCloakConfig config.KeyCloakConfig,
) {
	keyHandler := keys.New(temporalClient, tykClient, dbClient, omaClient, keyCloakClient, keyCloakConfig)
	server.POST("/keys", keyHandler.POST)
	server.GET("/keys", keyHandler.LIST)
	server.GET("/keys/:id", keyHandler.GET)
	server.PATCH("/keys/:id", keyHandler.PATCH)
	server.DELETE("/keys/:id", keyHandler.DELETE)

	policiesHandler := policiesHandler.New(tykClient)
	server.GET("/policies", policiesHandler.LIST)

	accountHandler := accounts.New(keyCloakClient, keyCloakConfig)
	server.GET("/accounts", accountHandler.GET)
}
