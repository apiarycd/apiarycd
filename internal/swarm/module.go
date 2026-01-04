package swarm

import (
	"github.com/go-core-fx/logger"
	"go.uber.org/fx"
)

func Module() fx.Option {
	return fx.Module(
		"swarm",
		logger.WithNamedLogger("swarm"),
		fx.Provide(NewSwarm),
	)
}
