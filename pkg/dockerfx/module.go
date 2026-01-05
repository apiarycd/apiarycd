package dockerfx

import (
	"context"
	"fmt"

	"github.com/go-core-fx/logger"
	"github.com/moby/moby/client"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func Module() fx.Option {
	return fx.Module(
		"dockerfx",
		logger.WithNamedLogger("dockerfx"),
		fx.Provide(NewClient),
		fx.Invoke(func(lc fx.Lifecycle, client *client.Client, logger *zap.Logger) {
			lc.Append(fx.Hook{
				OnStart: func(_ context.Context) error {
					logger.Info("starting docker module")
					return nil
				},
				OnStop: func(_ context.Context) error {
					logger.Info("stopping docker module")
					if err := client.Close(); err != nil {
						return fmt.Errorf("failed to close Docker client: %w", err)
					}
					return nil
				},
			})
		}),
	)
}
