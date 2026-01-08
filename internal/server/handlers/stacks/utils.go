package stacks

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func getStackID(c *fiber.Ctx) (uuid.UUID, error) {
	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return uuid.UUID{}, fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	return id, nil
}
