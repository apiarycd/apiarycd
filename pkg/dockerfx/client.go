package dockerfx

import (
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/moby/moby/client"
)

// NewClient creates a new Docker client with the given configuration.
func NewClient(cfg Config) (*client.Client, error) {
	if cfg.Timeout == 0 {
		cfg = DefaultConfig()
	}

	opts := []client.Opt{
		client.WithTimeout(cfg.Timeout),
		client.WithTLSClientConfigFromEnv(),
	}

	if cfg.Host != "" {
		opts = append(opts, client.WithHost(cfg.Host))
	}

	if cfg.APIVersion != "" {
		opts = append(opts, client.WithAPIVersion(cfg.APIVersion))
	}

	httpClient := &http.Client{
		Timeout: cfg.Timeout,
	}
	if cfg.TLSEnabled {
		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
		if cfg.TLSConfig.CAFile != "" || cfg.TLSConfig.CertFile != "" || cfg.TLSConfig.KeyFile != "" {
			// Load TLS certs if provided
			cert, err := tls.LoadX509KeyPair(cfg.TLSConfig.CertFile, cfg.TLSConfig.KeyFile)
			if err != nil {
				return nil, fmt.Errorf("failed to load TLS certificates: %w", err)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}

		httpClient.Transport = &http.Transport{
			TLSClientConfig: tlsConfig,
		}
	}
	opts = append(opts, client.WithHTTPClient(httpClient))

	cli, err := client.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return cli, nil
}
