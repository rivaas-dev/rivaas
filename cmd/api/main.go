package main

import (
	"flag"
	"fmt"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-api-key-manager/internal/config"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-api-key-manager/internal/handler"
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

	server := goskell.NewServer(gin.Logger(), gin.Recovery())
	server.WithLivez()

	keysHandler, err := handler.NewKeysHandlerWithConfiguration(&cfg.Config.Tyk)
	if err != nil {
		log.WithError(err).Fatal("could not create KeysHandler")
	}
	server.WithReadyZ(keysHandler)
	server.POST("/key", keysHandler.HandlePOST)

	if err = server.Run(fmt.Sprintf("%s:%d", cfg.Config.Application.Host, cfg.Config.Application.Port)); err != nil {
		log.WithError(err).Fatal("could not start the server")
	}
}
