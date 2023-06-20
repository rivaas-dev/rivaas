package main

import (
	"context"
	"fmt"
	"net/url"

	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/api/createkey"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/api/deletekey"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/api/getkey"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/api/listkey"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/api/listpolicy"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/api/updatekey"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/config"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/db"
	"gitlab.ci.fdmg.org/ci-api/tyk-sdk-go"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"gitlab.ci.fdmg.org/datacluster/nl/webservices/goconfig"
	"go.temporal.io/sdk/client"
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

	// Register handlers.
	registerHandlers(server, keysRepository, tykClient, temporalClient)

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
func registerHandlers(
	server *goskell.Server,
	dbClient db.DatabaseExecer,
	tykClient *tyk.APIClient,
	temporalClient client.Client,
) {
	createHandler := createkey.New(temporalClient, tykClient)
	server.POST("/keys", createHandler.Handle)

	getHandler := getkey.New(tykClient, dbClient)
	server.GET("/keys/:id", getHandler.Handle)

	updateHandler := updatekey.New(temporalClient, dbClient, tykClient)
	server.PATCH("/keys/:id", updateHandler.Handle)

	deleteHandler := deletekey.New(temporalClient, dbClient)
	server.DELETE("/keys/:id", deleteHandler.Handle)

	listHandler := listkey.New(dbClient)
	server.GET("/keys", listHandler.Handle)

	listpolicyHandler := listpolicy.New(tykClient)
	server.GET("/policies", listpolicyHandler.Handle)
}
