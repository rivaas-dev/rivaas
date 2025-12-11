package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/rs/zerolog/log"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/config"
	customerService "gitlab.ci.fdmg.org/ci-api/admin-api/internal/customers"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/db"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/handler/v1/accounts"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/handler/v1/keys"
	policiesHandler "gitlab.ci.fdmg.org/ci-api/admin-api/internal/handler/v1/policies"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/handler/v2/customers"
	accountsV2 "gitlab.ci.fdmg.org/ci-api/admin-api/internal/handler/v2/customers/accounts"
	keysV2 "gitlab.ci.fdmg.org/ci-api/admin-api/internal/handler/v2/keys"
	policiesV2 "gitlab.ci.fdmg.org/ci-api/admin-api/internal/handler/v2/policies"
	"gitlab.ci.fdmg.org/ci-api/admin-api/policies"
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/customer"
	"gitlab.ci.fdmg.org/ci-api/go-pkgs/solvimon"
	oma "gitlab.ci.fdmg.org/ci-api/oma/pkg/client"
	"gitlab.ci.fdmg.org/ci-api/tyk-sdk-go"
	"gitlab.ci.fdmg.org/datacluster/golibs/goot"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
	"go.companyinfo.dev/conflex"
	"go.companyinfo.dev/conflex/codec"
	"go.companyinfo.dev/keycloak"
	"go.opentelemetry.io/otel/propagation"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/contrib/opentelemetry"
	"go.temporal.io/sdk/interceptor"
)

const ProjectName = "admin_api"

func main() {
	if err := run(context.Background()); err != nil {
		log.Err(err).Msg("error while starting the application")
	}
}

func run(ctx context.Context) error {
	cfg, err := loadConfig(ctx)
	if err != nil {
		log.Fatal().Msgf("Failed to load config: %v", err)
	}

	// Debug logging: Show loaded pricing plans configuration
	log.Info().
		Int("pricingPlansCount", len(cfg.PricingPlans)).
		Msg("configuration loaded from file and Consul")

	for planID, plan := range cfg.PricingPlans {
		log.Debug().
			Str("planID", planID).
			Str("quotaPolicyName", plan.QuotaPolicyName).
			Int("numberOfAPIProductionKeys", plan.NumberOfAPIProductionKeys).
			Int("numberOfAPISandboxKeys", plan.NumberOfAPISandboxKeys).
			Msg("pricing plan in loaded config")
	}

	// initialize tracing
	goot.SetTraceProvider(ProjectName)
	tracer, err := goot.TracerProvider(ctx, cfg.OpenTelemetry.URL, ProjectName)
	if err != nil {
		return fmt.Errorf("failed to initialize tracing provider: %w", err)
	}
	defer func() {
		if err := tracer.Shutdown(context.Background()); err != nil {
			log.Err(err).Msg("error shutting down tracer provider")
		}
	}()
	// Create interceptor
	tp := tracer.Tracer(ProjectName)
	tracingInterceptor, err := opentelemetry.NewTracingInterceptor(opentelemetry.TracerOptions{Tracer: tp})
	if err != nil {
		log.Fatal().Msgf("Failed creating interceptor: %v", err)
	}

	// Initialize Goskell server.
	server := goskell.NewServer()
	server.WithMetrics("admin_api")
	server.WithTrace(
		ProjectName,
		goot.WithPropagators(propagation.TraceContext{}),
		goot.WithIgnoredPaths(goskell.IgnoredPathsList...),
		goot.WithTracerProvider(tracer),
	)

	// Connect to the database.
	keysRepository, err := db.New(
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Username,
		cfg.Database.Password,
		cfg.Database.Name,
		cfg.Database.SSLMode,
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
			HostPort:     cfg.Temporal.HostPort,
			Namespace:    cfg.Temporal.Namespace,
			Interceptors: []interceptor.ClientInterceptor{tracingInterceptor},
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
	customerClient := customer.NewService(keyCloakClient)

	// Create Solvimon client
	solvimonClient := solvimon.New(cfg.Solvimon.BaseUrl, cfg.Solvimon.ApiKey)

	// Connect to OMA and OPA
	omaClient := newOMAClient(ctx, cfg.OMA)

	// Register handlers.
	registerHandlers(server, keysRepository, tykClient, temporalClient, omaClient, keyCloakClient, keyCloakConfig, customerClient, solvimonClient, cfg)

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
	keyCloakClient *keycloak.Client,
	keyCloakConfig config.KeyCloakConfig,
	customerClient customer.CustomerClient,
	solvimonClient *solvimon.Client,
	cfg config.Config,
) {
	// v1 endpoints - not JSON:API compliant
	keyHandler := keys.New(temporalClient, tykClient, dbClient, omaClient, customerClient)
	server.POST("/keys", keyHandler.POST)
	server.GET("/keys", keyHandler.LIST)
	server.GET("/keys/:id", keyHandler.GET)
	server.PATCH("/keys/:id", keyHandler.PATCH)
	server.DELETE("/keys/:id", keyHandler.DELETE)

	policiesHandler := policiesHandler.New(tykClient)
	server.GET("/policies", policiesHandler.LIST)

	accountHandler := accounts.New(customerClient, solvimonClient)
	server.GET("/accounts", accountHandler.GET)
	server.PUT("/accounts/:id", accountHandler.PUT)

	// group v2 endpoints
	{
		customerSvc := customerService.New(dbClient, *keyCloakClient, cfg.PricingPlans)
		keyV2Handler := keysV2.New(temporalClient, tykClient, dbClient, omaClient, *keyCloakClient, keyCloakConfig, customerSvc, cfg.APIKeyDefaults, cfg.Pagination)

		v2 := server.GetRouter().Group("/v2")
		// keys endpoints
		v2.POST("/keys", keyV2Handler.POST)
		v2.GET("/keys", keyV2Handler.LIST)
		v2.GET("/keys/:id", keyV2Handler.GET)
		v2.PATCH("/keys/:id", keyV2Handler.PATCH)
		v2.DELETE("/keys/:id", keyV2Handler.DELETE)

		// policies endpoints
		policiesV2Handler := policiesV2.New(tykClient)
		v2.GET("/policies", policiesV2Handler.LIST)

		// customers endpoints
		customersV2Handler := customers.New(keyCloakClient, keyCloakConfig, omaClient, customerSvc, cfg.Pagination)
		v2.GET("/customers", customersV2Handler.LIST)
		v2.GET("/customers/:customerID", customersV2Handler.GET)

		// accounts endpoints
		accountsV2Handler := accountsV2.New(keyCloakClient, keyCloakConfig, solvimonClient, omaClient, customerSvc, cfg.Pagination, cfg.PricingPlans)
		v2.GET("/customers/:customerID/accounts", accountsV2Handler.LIST)
		v2.GET("/customers/:customerID/accounts/:accountID", accountsV2Handler.GET)
		v2.PUT("/customers/:customerID/accounts/:accountID", accountsV2Handler.PUT)
	}
}

func loadConfig(ctx context.Context) (config.Config, error) {
	var (
		cfg        config.Config
		conflexOpt []conflex.Option
	)

	conflexOpt = []conflex.Option{
		conflex.WithFileSource("config.yaml", codec.TypeYAML),
		conflex.WithBinding(&cfg),
	}
	if os.Getenv("CONSUL_HTTP_ADDR") != "" {
		conflexOpt = append(conflexOpt,
			conflex.WithConsulSource(os.Getenv("CONSUL_APP_KEY"), codec.TypeYAML),
			conflex.WithConsulSource(os.Getenv("CONSUL_PRICING_PLANS_KEY"), codec.TypeYAML),
		)
	}
	cnflx, err := conflex.New(conflexOpt...)
	if err != nil {
		return cfg, fmt.Errorf("failed to initialize config provider: %w", err)
	}

	err = cnflx.Load(ctx)
	if err != nil {
		return cfg, fmt.Errorf("failed to read config: %w", err)
	}

	return cfg, nil
}
