package config

import (
	"github.com/spf13/viper"
	"sync"
)

func LoadConfig(path string) (*TYKConfiguration, error) {
	Config = &TYKConfiguration{
		viper: viper.New(),
		lock:  sync.Mutex{},
	}

	var err error

	Config.viper.AddConfigPath(path)
	Config.viper.SetConfigName("config")
	Config.viper.SetConfigType("yml")

	Config.viper.AutomaticEnv()

	err = Config.viper.ReadInConfig()
	if err != nil {
		return Config, err
	}

	err = Config.viper.Unmarshal(&Config.Config)

	return Config, err
}
