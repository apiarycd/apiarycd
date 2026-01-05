package dockerfx

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"

	"github.com/moby/moby/client"
)

// NewClient creates a new Docker client with the given configuration.
func NewClient(cfg Config) (*client.Client, error) {
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultConfig().Timeout
	}

	opts := []client.Opt{
		client.WithTimeout(cfg.Timeout),
		client.WithTLSClientConfigFromEnv(),
		client.WithHostFromEnv(),
		client.WithAPIVersionFromEnv(),
	}

	if cfg.Host != "" {
		opts = append(opts, client.WithHost(cfg.Host))
	}

	if cfg.APIVersion != "" {
		opts = append(opts, client.WithAPIVersion(cfg.APIVersion))
	}

	if cfg.TLSEnabled {
		httpClient := &http.Client{
			Timeout: cfg.Timeout,
		}
		tlsConfig, err := newTLSConfig(cfg)
		if err != nil {
			return nil, err
		}

		httpClient.Transport = &http.Transport{
			TLSClientConfig: tlsConfig,
		}
		opts = append(opts, client.WithHTTPClient(httpClient))
	}

	cli, err := client.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return cli, nil
}

func newTLSConfig(cfg Config) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}
	if cfg.TLSConfig.CAFile != "" {
		caCert, err := os.ReadFile(cfg.TLSConfig.CAFile)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to read CA certificate: %w", ErrInvalidTLSConfig, err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("%w: failed to parse CA certificate", ErrInvalidTLSConfig)
		}
		tlsConfig.RootCAs = caCertPool
	}

	// Load TLS certs if provided
	if cfg.TLSConfig.CertFile != "" && cfg.TLSConfig.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.TLSConfig.CertFile, cfg.TLSConfig.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to load TLS certificates: %w", ErrInvalidTLSConfig, err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}
