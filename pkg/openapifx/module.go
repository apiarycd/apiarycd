package openapifx

import (
	"github.com/go-core-fx/logger"
	"go.uber.org/fx"
)

func Module() fx.Option {
	return fx.Module(
		"openapifx",
		logger.WithNamedLogger("openapifx"),
		fx.Provide(New),
	)
}
