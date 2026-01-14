package openapifx

import (
	"sync"

	"github.com/go-core-fx/healthfx"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/etag"
	"github.com/gofiber/swagger"
	"github.com/swaggo/swag"
)

type Handler struct {
	config  Config
	version healthfx.Version
	docs    *swag.Spec

	once sync.Once
}

func New(config Config, version healthfx.Version, docs *swag.Spec) *Handler {
	return &Handler{
		config:  config,
		version: version,
		docs:    docs,

		once: sync.Once{},
	}
}

func (h *Handler) Register(router fiber.Router) {
	if !h.config.Enabled {
		return
	}

	h.docs.Version = h.version.Version
	h.docs.Host = h.config.PublicHost
	h.docs.BasePath = h.config.PublicPath

	router.Use("*",
		// Pre-middleware: set host/scheme dynamically
		func(c *fiber.Ctx) error {
			h.once.Do(func() {
				if h.docs.Host == "" {
					h.docs.Host = c.Hostname()
				}

				h.docs.Schemes = []string{c.Protocol()}
			})

			return c.Next()
		},
		etag.New(etag.Config{Weak: true}),
		swagger.New(swagger.Config{Layout: "BaseLayout", URL: "doc.json"}),
	)
}
