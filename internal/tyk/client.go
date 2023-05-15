package tyk

import "gitlab.ci.fdmg.org/ci-api/tyk-sdk-go"

type Config struct {
	Endpoint string `mapstructure:"endpoint"`
	Scheme   string `mapstructure:"scheme"`
	Debug    bool   `mapstructure:"debug"`
	Secret   string `mapstructure:"secret"`
}

func NewClient(configuration Config) *tyk.APIClient {
	return tyk.NewAPIClient(&tyk.Configuration{
		Host:          configuration.Endpoint,
		Scheme:        configuration.Scheme,
		DefaultHeader: map[string]string{"x-tyk-authorization": configuration.Secret},
		Debug:         configuration.Debug,
	})
}
