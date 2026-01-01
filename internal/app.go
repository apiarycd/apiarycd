package internal

import (
	"context"

	"github.com/go-core-fx/logger"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func Run() {
	fx.New(
		// CORE MODULES
		logger.Module(),
		logger.WithFxDefaultLogger(),
		// sqlfx.Module(),
		// goosefx.Module(),
		// bunfx.Module(),
		// fiberfx.Module(),
		//
		// APP MODULES
		// config.Module(),
		// db.Module(),
		// server.Module(),
		// bot.Module(),
		//
		// BUSINESS MODULES
		// users.Module(),
		//
		fx.Invoke(func(lc fx.Lifecycle, logger *zap.Logger) {
			lc.Append(fx.Hook{
				OnStart: func(_ context.Context) error {
					logger.Info("app started")
					return nil
				},
				OnStop: func(_ context.Context) error {
					logger.Info("app stopped")
					return nil
				},
			})
		}),
	).Run()
}
