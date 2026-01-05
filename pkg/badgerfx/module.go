package badgerfx

import (
	"context"
	"fmt"

	"github.com/dgraph-io/badger/v4"
	"github.com/go-core-fx/logger"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func Module() fx.Option {
	return fx.Module(
		"badgerfx",
		logger.WithNamedLogger("badgerfx"),
		fx.Provide(newLogger, fx.Private),
		fx.Provide(New),
		fx.Invoke(func(db *badger.DB, logger *zap.Logger, lifecycle fx.Lifecycle) {
			lifecycle.Append(fx.Hook{
				OnStart: func(_ context.Context) error {
					logger.Info("starting badger module")
					return nil
				},
				OnStop: func(_ context.Context) error {
					logger.Info("stopping badger module")
					if err := db.Close(); err != nil {
						return fmt.Errorf("failed to close BadgerDB: %w", err)
					}
					return nil
				},
			})
		}),
	)
}
