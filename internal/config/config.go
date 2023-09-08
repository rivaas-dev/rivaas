package config

// Config represents application configuration.
type Config struct {
	Host     string
	Port     uint16
	LogLevel string
	Database Database
	Tyk      Tyk
	Temporal Temporal
	OMA      OMA
}

// Database represents Database configuration.
type Database struct {
	Host     string
	Port     uint16
	Username string
	Password string
	Name     string
}

// Tyk represents Tyk configuration.
type Tyk struct {
	URL    string
	Secret string
	Debug  bool
}

// Temporal represents Temporal configuration.
type Temporal struct {
	HostPort  string
	Namespace string
}

// OMA represents OMA configuration.
type OMA struct {
	Address   string
	Namespace string
	OPA       OPA
}

// OPA represents OPA configuration.
type OPA struct {
	Address string
}
