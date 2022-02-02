package config

import (
	"errors"
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
	if err != nil {
		return Config, err
	}
	err = validateConfig(Config)

	return Config, err
}

//validateConfig simple validation step, add more if needed
func validateConfig(config *TYKConfiguration) error {
	if config.Config.Tyk.Policies == nil || len(config.Config.Tyk.Policies) < 1 {
		return errors.New("did not find any policies to serve")
	}

	return nil
}
