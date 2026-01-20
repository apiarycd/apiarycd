package config

import (
	"fmt"
	"os"
	"time"

	"github.com/go-core-fx/config"
)

type http struct {
	Address     string   `koanf:"address"`
	ProxyHeader string   `koanf:"proxy_header"`
	Proxies     []string `koanf:"proxies"`

	OpenAPI openAPIConfig `koanf:"openapi"`
}

type openAPIConfig struct {
	Enabled    bool   `koanf:"enabled"`
	PublicHost string `koanf:"public_host"`
	PublicPath string `koanf:"public_path"`
}

type storageConfig struct {
	DataDir string `koanf:"data_dir"`
}

type dockerConfig struct {
	Host       string        `koanf:"host"`
	APIVersion string        `koanf:"api_version"`
	Timeout    time.Duration `koanf:"timeout"`
	TLSEnabled bool          `koanf:"tls_enabled"`
	CAFile     string        `koanf:"ca_file"`
	CertFile   string        `koanf:"cert_file"`
	KeyFile    string        `koanf:"key_file"`
}

type gitAuthConfig struct {
	SSH   gitSSHAuthConfig   `koanf:"ssh"`
	HTTPS gitHTTPSAuthConfig `koanf:"https"`
}

type gitSSHAuthConfig struct {
	DefaultPrivateKey string `koanf:"default_private_key"`
}

type gitHTTPSAuthConfig struct {
	DefaultToken    string `koanf:"default_token"`
	DefaultUsername string `koanf:"default_username"`
}

type gitConfig struct {
	Timeout    time.Duration `koanf:"timeout"`
	DefaultDir string        `koanf:"default_dir"`
	Auth       gitAuthConfig `koanf:"auth"`
}

type Config struct {
	HTTP http `koanf:"http"`

	Storage storageConfig `koanf:"storage"`
	Docker  dockerConfig  `koanf:"docker"`
	Git     gitConfig     `koanf:"git"`
}

func Default() Config {
	//nolint:exhaustruct,mnd //default values
	return Config{
		HTTP: http{
			Address:     "127.0.0.1:3000",
			ProxyHeader: "X-Forwarded-For",
			Proxies:     []string{},
		},

		Storage: storageConfig{
			DataDir: "./data",
		},

		Docker: dockerConfig{
			Host:       "",
			APIVersion: "",
			Timeout:    30 * time.Second,
		},

		Git: gitConfig{
			Timeout:    30 * time.Second,
			DefaultDir: "./repos",
		},
	}
}

func New() (Config, error) {
	cfg := Default()

	options := []config.Option{}
	if yamlPath := os.Getenv("CONFIG_PATH"); yamlPath != "" {
		options = append(options, config.WithLocalYAML(yamlPath))
	}

	if err := config.Load(&cfg, options...); err != nil {
		return Config{}, fmt.Errorf("failed to load config: %w", err)
	}

	return cfg, nil
}
