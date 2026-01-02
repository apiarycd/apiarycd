package internal

import (
	"context"

	"github.com/apiarycd/apiarycd/internal/config"
	"github.com/apiarycd/apiarycd/internal/example"
	"github.com/apiarycd/apiarycd/internal/server"
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
		config.Module(),
		// db.Module(),
		server.Module(),
		// bot.Module(),
		//
		// BUSINESS MODULES
		example.Module(),
		//
		// LIFECYCLE MANAGEMENT
		fx.Invoke(func(lc fx.Lifecycle, logger *zap.Logger) {
			lc.Append(fx.Hook{
				OnStart: func(_ context.Context) error {
					logger.Info("🚀 ApiaryCD application starting up")
					return nil
				},
				OnStop: func(_ context.Context) error {
					logger.Info("🛑 ApiaryCD application shutting down gracefully")
					return nil
				},
			})
		}),
	).Run()
}
