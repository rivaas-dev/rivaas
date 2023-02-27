package config

import (
	"github.com/spf13/viper"
	"gitlab.ci.fdmg.org/datacluster/germany/api-gateway/tyk-api-key-manager/internal/tyk"
	"sync"
)

var Config *TYKConfiguration

type TYKConfiguration struct {
	viper  *viper.Viper
	Config Configuration
	lock   sync.Mutex
}

type Configuration struct {
	Application Application `mapstructure:"application"`
	Tyk         tyk.Config  `mapstructure:"tyk"`
	Database    Database    `mapstructure:"database"`
}

type Database struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Name     string `mapstructure:"name"`
}

type Application struct {
	Host     string `mapstructure:"host"`
	LogLevel string `mapstructure:"logLevel"`
	Port     int64  `mapstructure:"port"`
}

func (cfg *TYKConfiguration) Fetch() error {
	cfg.lock.Lock()
	defer cfg.lock.Unlock()

	err := cfg.viper.ReadRemoteConfig()
	if err != nil {
		return err
	}
	return cfg.viper.Unmarshal(&cfg.Config)
}
