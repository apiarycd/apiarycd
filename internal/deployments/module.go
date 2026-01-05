package deployments

import (
	"github.com/go-core-fx/logger"
	"go.uber.org/fx"
)

func Module() fx.Option {
	return fx.Module(
		"deployments",
		logger.WithNamedLogger("deployments"),
		fx.Provide(NewRepository, fx.Private),
		fx.Provide(NewService),
	)
}
