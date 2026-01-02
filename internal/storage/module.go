package storage

import (
	"context"

	"github.com/go-core-fx/logger"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// Module returns an Fx option for the storage module.
func Module() fx.Option {
	return fx.Module(
		"storage",
		logger.WithNamedLogger("storage"),

		fx.Provide(
			New,
		),

		fx.Invoke(func(lc fx.Lifecycle, storage Storage, logger *zap.Logger) error {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					logger.Info("starting storage module")
					return storage.Initialize(ctx)
				},
				OnStop: func(ctx context.Context) error {
					logger.Info("stopping storage module")
					return storage.Close(ctx)
				},
			})
			return nil
		}),
	)
}
