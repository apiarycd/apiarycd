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

type PerformanceConfig struct {
	MaxConcurrentOperations int
	MaxRepositorySizeBytes  int64
	MinDiskSpaceBytes       int64
	RetryAttempts           int
	DefaultTimeout          time.Duration
}

type Config struct {
	Timeout     time.Duration
	DefaultDir  string
	Auth        AuthConfig
	Performance PerformanceConfig
}
