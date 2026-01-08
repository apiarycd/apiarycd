package validation

import (
	"github.com/go-core-fx/fiberfx"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

type localKey string

const (
	localBody localKey = "validated_body"
)

// NewBody validates the request body and stores the validated request in the context.
func NewBody[T any](validate *validator.Validate) fiber.Handler {
	return func(c *fiber.Ctx) error {
		req, err := ValidateBody[T](c, validate)
		if err != nil {
			return err
		}

		// Store the validated request in the context for later use
		c.Locals(localBody, req)
		return c.Next()
	}
}

// ValidateBody validates the request body.
func ValidateBody[T any](c *fiber.Ctx, validate *validator.Validate) (*T, error) {
	var req T
	if err := c.BodyParser(&req); err != nil {
		return nil, NewErrors(err)
	}

	if err := validate.Var(&req, "required,dive"); err != nil {
		return nil, NewErrors(err)
	}

	return &req, nil
}

// GetValidatedBody retrieves the validated request from the context.
func GetValidatedBody[T any](c *fiber.Ctx) (*T, bool) {
	b, ok := c.Locals(localBody).(*T)
	return b, ok
}

// DecorateWithBody decorates a handler with the validated request.
func DecorateWithBody[T any](h func(c *fiber.Ctx, body *T) error) fiber.Handler {
	return func(c *fiber.Ctx) error {
		body, ok := GetValidatedBody[T](c)
		if !ok {
			return c.Status(fiber.StatusBadRequest).JSON(
				fiberfx.NewErrorResponse(
					"invalid request body",
					fiber.StatusBadRequest,
					nil,
				),
			)
		}

		return h(c, body)
	}
}

// DecorateWithBodyEx decorates a handler with the validated request.
func DecorateWithBodyEx[T any](validate *validator.Validate, h func(c *fiber.Ctx, body *T) error) fiber.Handler {
	return func(c *fiber.Ctx) error {
		body, err := ValidateBody[T](c, validate)
		if err != nil {
			return err
		}

		return h(c, body)
	}
}
