package validation

import (
	"github.com/go-core-fx/fiberfx"
	"github.com/gofiber/fiber/v2"
	"github.com/samber/lo"
)

func Middleware(c *fiber.Ctx) error {
	err := c.Next()
	if err == nil {
		return nil
	}

	if err, ok := lo.ErrorsAs[Errors](err); ok {
		return c.Status(fiber.StatusBadRequest).JSON(
			fiberfx.NewErrorResponse(
				err.Error(),
				fiber.StatusBadRequest,
				err,
			),
		)
	}

	return err //nolint:wrapcheck // pass to upstream middleware
}
