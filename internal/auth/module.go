package auth

import (
	"github.com/go-core-fx/logger"
	"go.uber.org/fx"
)

func Module() fx.Option {
	return fx.Module(
		"auth",
		logger.WithNamedLogger("auth"),
	)
}
