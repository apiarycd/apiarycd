package dockerfx

import (
	"time"
)

// Config holds the configuration for the Docker client.
//
// This struct contains all the configuration parameters needed to initialize
// and configure the Docker client. These values can be loaded from
// environment variables, configuration files, or other sources.
//
// Example:
//
//	cfg := dockerfx.Config{
//	    Host:    "unix:///var/run/docker.sock",
//	    APIVersion: "1.40",
//	    Timeout: 30 * time.Second,
//	}
//
//	client := dockerfx.NewClient(cfg)
type Config struct {
	// Host specifies the Docker daemon host. It can be a Unix socket path
	// (e.g., "unix:///var/run/docker.sock") or a TCP address
	// (e.g., "tcp://127.0.0.1:2376"). Defaults to the Docker environment.
	Host string

	// APIVersion specifies the API version to use. If empty, the client will
	// negotiate the version with the daemon.
	APIVersion string

	// Timeout specifies the timeout for Docker API requests.
	// Defaults to 30 seconds if zero.
	Timeout time.Duration

	// TLSEnabled enables TLS for the connection.
	TLSEnabled bool

	// TLSConfig provides TLS configuration for secure connections.
	// Only used if TLSEnabled is true.
	TLSConfig TLSConfig
}

// TLSConfig holds TLS configuration for Docker client.
type TLSConfig struct {
	// CAFile path to the CA certificate file.
	CAFile string

	// CertFile path to the client certificate file.
	CertFile string

	// KeyFile path to the client key file.
	KeyFile string
}

// DefaultConfig returns a default configuration for the Docker client.
func DefaultConfig() Config {
	//nolint:exhaustruct,mnd //default values
	return Config{
		Host:       "", // Use Docker environment default
		APIVersion: "",
		Timeout:    30 * time.Second,
	}
}
