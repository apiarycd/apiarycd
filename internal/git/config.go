package git

import "time"

type AuthConfig struct {
	SSH   SSHAuthConfig
	HTTPS HTTPSAuthConfig
}

type SSHAuthConfig struct {
	DefaultPrivateKey string
}

type HTTPSAuthConfig struct {
	DefaultToken    string
	DefaultUsername string
}

type Config struct {
	Timeout    time.Duration
	DefaultDir string
	Auth       AuthConfig
}
