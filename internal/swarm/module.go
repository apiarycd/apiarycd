package swarm

import (
	"github.com/apiarycd/apiarycd/internal/swarm/immutables"
	"github.com/go-core-fx/logger"
	"go.uber.org/fx"
)

func Module() fx.Option {
	return fx.Module(
		"swarm",
		logger.WithNamedLogger("swarm"),
		fx.Provide(NewSwarm),
		fx.Provide(immutables.NewRenderer),
	)
}
