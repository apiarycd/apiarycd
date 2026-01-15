package server

import (
	"github.com/apiarycd/apiarycd/internal/server/docs"
	"github.com/apiarycd/apiarycd/internal/server/handlers/stacks"
	"github.com/apiarycd/apiarycd/pkg/openapifx"
	"github.com/go-core-fx/fiberfx"
	"github.com/go-core-fx/fiberfx/handler"
	"github.com/go-core-fx/fiberfx/health"
	"github.com/go-core-fx/fiberfx/validation"
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
		fx.Supply(docs.SwaggerInfo),

		fx.Provide(
			fx.Annotate(health.NewHandler, fx.ResultTags(`name:"health-handler"`)), fx.Private,
			fx.Annotate(stacks.NewHandler, fx.ResultTags(`group:"handlers"`)), fx.Private,
		),

		fx.Invoke(
			fx.Annotate(
				func(handlers []handler.Handler, healthHandler handler.Handler, openapiHandler *openapifx.Handler, app *fiber.App) {
					// Health endpoint
					healthHandler.Register(app)

					// Version 1 API group
					v1 := app.Group("/api/v1")
					openapiHandler.Register(v1.Group("/docs"))

					v1.Use(validation.Middleware)

					for _, h := range handlers {
						h.Register(v1)
					}
				},
				fx.ParamTags(`group:"handlers"`, `name:"health-handler"`),
			),
		),
	)
}
