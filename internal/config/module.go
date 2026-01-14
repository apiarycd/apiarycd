package config

import (
	"github.com/apiarycd/apiarycd/pkg/badgerfx"
	"github.com/apiarycd/apiarycd/pkg/dockerfx"
	"github.com/apiarycd/apiarycd/pkg/openapifx"
	"github.com/go-core-fx/fiberfx"
	"go.uber.org/fx"
)

func Module() fx.Option {
	return fx.Module(
		"config",
		fx.Provide(New),
		fx.Provide(func(cfg Config) fiberfx.Config {
			return fiberfx.Config{
				Address:     cfg.HTTP.Address,
				ProxyHeader: cfg.HTTP.ProxyHeader,
				Proxies:     cfg.HTTP.Proxies,
			}
		}),
		fx.Provide(func(cfg Config) badgerfx.Config {
			return badgerfx.Config{
				Dir: cfg.Storage.DataDir,
			}
		}),
		fx.Provide(func(cfg Config) dockerfx.Config {
			return dockerfx.Config{
				Host:       cfg.Docker.Host,
				APIVersion: cfg.Docker.APIVersion,
				Timeout:    cfg.Docker.Timeout,
				TLSEnabled: cfg.Docker.TLSEnabled,
				TLSConfig: dockerfx.TLSConfig{
					CAFile:   cfg.Docker.CAFile,
					CertFile: cfg.Docker.CertFile,
					KeyFile:  cfg.Docker.KeyFile,
				},
			}
		}),
		fx.Provide(func(cfg Config) openapifx.Config {
			return openapifx.Config{
				Enabled:    cfg.HTTP.OpenAPI.Enabled,
				PublicHost: cfg.HTTP.OpenAPI.PublicHost,
				PublicPath: cfg.HTTP.OpenAPI.PublicPath,
			}
		}),
	)
}
