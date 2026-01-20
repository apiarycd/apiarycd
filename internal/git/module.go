package git

import (
	"github.com/go-core-fx/logger"
	"go.uber.org/fx"
)

func Module() fx.Option {
	return fx.Module(
		"git",
		logger.WithNamedLogger("git"),
		fx.Provide(NewService),
	)
}
