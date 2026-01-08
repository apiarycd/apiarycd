package server

import (
	"reflect"
	"strings"

	_ "github.com/apiarycd/apiarycd/internal/server/docs" // This is required for swagger docs
	"github.com/apiarycd/apiarycd/internal/server/handlers/stacks"
	"github.com/apiarycd/apiarycd/internal/server/validation"
	"github.com/go-core-fx/fiberfx"
	"github.com/go-core-fx/fiberfx/handler"
	"github.com/go-core-fx/fiberfx/health"
	"github.com/go-core-fx/logger"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	fiberSwagger "github.com/swaggo/fiber-swagger"
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

		fx.Decorate(func(v *validator.Validate) *validator.Validate {
			v.RegisterTagNameFunc(func(fld reflect.StructField) string {
				//nolint:mnd //fixed length
				name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
				if name == "-" {
					return ""
				}
				return name
			})
			return v
		}),
		fx.Provide(
			fx.Annotate(health.NewHandler, fx.ResultTags(`group:"handlers"`)), fx.Private,
			fx.Annotate(stacks.NewHandler, fx.ResultTags(`group:"handlers"`)), fx.Private,
		),

		fx.Invoke(
			fx.Annotate(
				func(handlers []handler.Handler, app *fiber.App) {
					// Swagger documentation
					app.Get("/swagger/*", fiberSwagger.WrapHandler)

					// Version 1 API group
					v1 := app.Group("/api/v1")
					v1.Use(validation.Middleware)

					for _, h := range handlers {
						h.Register(v1)
					}
				},
				fx.ParamTags(`group:"handlers"`),
			),
		),
	)
}
