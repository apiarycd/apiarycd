package repositories

import (
	"github.com/go-core-fx/logger"
	"go.uber.org/fx"
)

func Module() fx.Option {
	return fx.Module(
		"repositories",
		logger.WithNamedLogger("repositories"),
		fx.Provide(NewService),
	)
}
