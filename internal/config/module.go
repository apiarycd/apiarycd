package config

import (
	"github.com/apiarycd/apiarycd/internal/deployments"
	"github.com/apiarycd/apiarycd/internal/repositories"
	"github.com/apiarycd/apiarycd/pkg/badgerfx"
	"github.com/apiarycd/apiarycd/pkg/dockerfx"
	"github.com/go-core-fx/fiberfx"
	"github.com/go-core-fx/fiberfx/openapi"
	"github.com/samber/lo"
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
		fx.Provide(func(cfg Config) openapi.Config {
			return openapi.Config{
				Enabled:    cfg.HTTP.OpenAPI.Enabled,
				PublicHost: cfg.HTTP.OpenAPI.PublicHost,
				PublicPath: cfg.HTTP.OpenAPI.PublicPath,
			}
		}),

		fx.Provide(
			func(cfg Config) repositories.Config {
				return repositories.Config{
					Timeout:    cfg.Repositories.Timeout,
					StorageDir: cfg.Repositories.StorageDir,
					Auth: repositories.GitAuth{
						SSH: &repositories.GitSSHAuth{
							PrivateKeyPath: cfg.Repositories.DefaultAuth.SSH.PrivateKeyPath,
							Username:       lo.CoalesceOrEmpty(cfg.Repositories.DefaultAuth.SSH.Username, "git"),
							Password:       "",
						},
						HTTPS: &repositories.GitHTTPSAuth{
							Username: cfg.Repositories.DefaultAuth.HTTPS.Username,
							Password: cfg.Repositories.DefaultAuth.HTTPS.Password,
						},
					},
				}
			},
			func(cfg Config) deployments.Config {
				return deployments.Config{
					DeployTimeout:            cfg.Deployments.DeployTimeout,
					RotateImmutableResources: cfg.Deployments.RotateImmutableResources,
				}
			},
		),
	)
}
