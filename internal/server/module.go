package server

import (
	"github.com/go-core-fx/fiberfx"
	"github.com/go-core-fx/fiberfx/handler"
	"github.com/go-core-fx/fiberfx/health"
	"github.com/go-core-fx/logger"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func Module() fx.Option {
	return fx.Module(
		"server",
		logger.WithNamedLogger("server"),

		fx.Provide(func(log *zap.Logger) fiberfx.Options {
			opts := fiberfx.Options{}
			opts.WithErrorHandler(fiberfx.NewJSONErrorHandler(log))
			opts.WithMetrics()
			return opts
		}),

		fx.Provide(
			fx.Annotate(health.NewHandler, fx.ResultTags(`group:"handlers"`)), fx.Private,
		),

		fx.Invoke(
			fx.Annotate(
				func(handlers []handler.Handler, app *fiber.App) {
					for _, h := range handlers {
						h.Register(app)
					}
				},
				fx.ParamTags(`group:"handlers"`),
			),
		),

		fx.Invoke(SetupRoutes),
	)
}
