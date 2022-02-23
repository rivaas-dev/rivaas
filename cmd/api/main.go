package main

import (
	"flag"
	"fmt"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-api-key-manager/internal/config"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-api-key-manager/internal/key"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-api-key-manager/internal/policies"
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
	keysRepository, err := key.NewSQLRepositoryFromCredentials(dbConfig.Address, dbConfig.Username, dbConfig.Password,
		dbConfig.Name)
	if err != nil {
		panic(err)
	}

	server := goskell.NewServer(gin.Logger(), gin.Recovery())
	policyHandler := policies.NewHandler(cfg.Config.Tyk.Policies)
	keysHandler := key.NewHandlerFromConfiguration(&cfg.Config.Tyk, keysRepository, cfg.Config.Tyk.Policies)
	server.POST("/keys", keysHandler.HandlePOST)
	server.GET("/keys/:"+key.HashPathName, keysHandler.HandleGETKey)
	server.GET("/policies", policyHandler.GetPolicy)

	if err = server.Run(fmt.Sprintf("%s:%d", cfg.Config.Application.Host, cfg.Config.Application.Port)); err != nil {
		log.WithError(err).Fatal("could not start the server")
	}
}
