package config

import (
	"errors"
	log "github.com/sirupsen/logrus"
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
	printConfig(Config)

	return Config, err
}

// validateConfig simple validation step, add more if needed
func validateConfig(config *TYKConfiguration) error {
	if config.Config.Database.Username == "" || config.Config.Database.Password == "" ||
		config.Config.Database.Host == "" || config.Config.Database.Port == 0 || config.Config.Database.Name == "" {
		return errors.New("database not configured properly")
	}

	return nil
}

func printConfig(config *TYKConfiguration) {
	fields := log.Fields{
		"database.username": config.Config.Database.Username,
		"database.host":     config.Config.Database.Host,
		"database.port":     config.Config.Database.Port,
		"database.name":     config.Config.Database.Name,
		"tyk.endpoint":      config.Config.Tyk.Endpoint,
	}

	log.WithFields(fields).Info()
}
