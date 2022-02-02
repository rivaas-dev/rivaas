package config

import (
	"github.com/spf13/viper"
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
	Tyk         Tyk         `mapstructure:"tyk"`
}

type Application struct {
	Host     string `mapstructure:"host"`
	LogLevel string `mapstructure:"logLevel"`
	Port     int64  `mapstructure:"port"`
}

type Tyk struct {
	Endpoint string   `mapstructure:"endpoint"`
	Scheme   string   `mapstructure:"scheme"`
	Debug    bool     `mapstructure:"debug"`
	Secret   string   `mapstructure:"secret"`
	Policies []string `mapstructure:"policies"`
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
