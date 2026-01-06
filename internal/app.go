package internal

import (
	"context"

	"github.com/apiarycd/apiarycd/internal/config"
	"github.com/apiarycd/apiarycd/internal/deployments"
	"github.com/apiarycd/apiarycd/internal/server"
	"github.com/apiarycd/apiarycd/internal/stacks"
	"github.com/apiarycd/apiarycd/internal/swarm"
	"github.com/apiarycd/apiarycd/pkg/badgerfx"
	"github.com/apiarycd/apiarycd/pkg/dockerfx"
	"github.com/capcom6/go-infra-fx/validator"
	"github.com/go-core-fx/fiberfx"
	"github.com/go-core-fx/fiberfx/health"
	"github.com/go-core-fx/healthfx"
	"github.com/go-core-fx/logger"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func Run() {
	fx.New(
		// CORE MODULES
		logger.Module(),
		logger.WithFxDefaultLogger(),
		badgerfx.Module(),
		dockerfx.Module(),
		healthfx.Module(),
		fiberfx.Module(),
		validator.Module,
		//
		// APP MODULES
		config.Module(),
		server.Module(),
		swarm.Module(),
		//
		// BUSINESS MODULES
		fx.Provide(func() health.Version { return health.Version{Version: "0.0.1", ReleaseID: 1} }),
		stacks.Module(),
		deployments.Module(),
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
