package stacks

import (
	"github.com/go-core-fx/logger"
	"go.uber.org/fx"
)

func Module() fx.Option {
	return fx.Module(
		"stacks",
		logger.WithNamedLogger("stacks"),
		fx.Provide(NewRepository, fx.Private),
		fx.Provide(NewService),
	)
}
