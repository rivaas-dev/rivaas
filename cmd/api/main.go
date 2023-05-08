package main

import (
	"flag"
	"fmt"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/config"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/key"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/policy"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/tyk"
	"gitlab.ci.fdmg.org/datacluster/golibs/goskell"
)

func main() {
	configPath := flag.String("config", "./", "main application configuration file path")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		panic(err)
	}

	lvl, err := log.ParseLevel(cfg.Config.Application.LogLevel)
	if err != nil {
		panic(err)
	}

	logger := log.New()
	logger.SetLevel(lvl)

	dbConfig := cfg.Config.Database
	keysRepository, err := key.NewSQLRepositoryFromCredentials(
		dbConfig.Host,
		dbConfig.Port,
		dbConfig.Username,
		dbConfig.Password,
		dbConfig.Name,
	)
	if err != nil {
		panic(err)
	}

	server := goskell.NewServer(gin.Logger(), gin.Recovery())

	tykClient := tyk.NewClient(cfg.Config.Tyk)

	policyHandler := policy.NewHandler(tykClient.PoliciesApi)
	keysHandler := key.NewHandler(tykClient.KeysApi, tykClient.PoliciesApi, keysRepository)

	server.POST("/keys", keysHandler.HandlePOST)
	server.GET("/keys", keysHandler.HandleGETKeys)
	server.GET("/keys/:"+key.HashPathName, keysHandler.HandleGETKey)
	server.PATCH("/keys/:"+key.HashPathName, keysHandler.HandlePATCHKey)
	server.GET("/policies", policyHandler.HandleGETPolicies)

	if err = server.Run(fmt.Sprintf("%s:%d", cfg.Config.Application.Host, cfg.Config.Application.Port)); err != nil {
		log.WithError(err).Fatal("could not start the server")
	}
}
