package stacks

import (
	"errors"
	"fmt"

	"github.com/apiarycd/apiarycd/internal/server/validation"
	"github.com/apiarycd/apiarycd/internal/stacks"
	"github.com/go-core-fx/fiberfx/handler"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Handler struct {
	stacksSvc *stacks.Service

	validator *validator.Validate
	logger    *zap.Logger
}

func NewHandler(stacksSvc *stacks.Service, validator *validator.Validate, logger *zap.Logger) handler.Handler {
	return &Handler{
		stacksSvc: stacksSvc,

		validator: validator,
		logger:    logger,
	}
}

// Register implements handler.Handler.
func (h *Handler) Register(r fiber.Router) {
	r = r.Group("/stacks")

	r.Use(h.errorsHandler)
	r.Post("/", validation.DecorateWithBodyEx(h.validator, h.post))
	r.Get("/", h.list)
	r.Get("/:id", h.get)
	r.Patch("/:id", validation.DecorateWithBodyEx(h.validator, h.patch))
	r.Delete("/:id", h.delete)
}

//	@Summary		Create a new stack
//	@Description	Create a new Docker Swarm stack with the provided configuration
//	@Tags			stacks
//	@Accept			json
//	@Produce		json
//	@Param			stack	body		CreateRequest	true	"Stack creation request"
//	@Success		201		{object}	StackResponse
//	@Failure		400		{object}	fiberfx.ErrorResponse
//	@Failure		409		{object}	fiberfx.ErrorResponse
//	@Router			/stacks [post]
//
// Create a new stack.
func (h *Handler) post(c *fiber.Ctx, req *CreateRequest) error {
	draft := &stacks.StackDraft{
		Name:        req.Name,
		Description: req.Description,
		GitURL:      req.GitURL,
		GitBranch:   req.GitBranch,
		ComposePath: req.ComposePath,
		Variables:   req.Variables,
		AutoDeploy:  req.AutoDeploy,
		Labels:      req.Labels,
	}

	stack, err := h.stacksSvc.Create(c.Context(), draft)
	if err != nil {
		return fmt.Errorf("failed to create stack: %w", err)
	}

	response := h.toResponse(stack)
	return c.Status(fiber.StatusCreated).JSON(response)
}

//	@Summary		List all stacks
//	@Description	Retrieve a list of all configured stacks
//	@Tags			stacks
//	@Accept			json
//	@Produce		json
//	@Success		200	{array}	StackResponse
//	@Router			/stacks [get]
//
// List all stacks.
func (h *Handler) list(c *fiber.Ctx) error {
	stacks, err := h.stacksSvc.List(c.Context())
	if err != nil {
		return fmt.Errorf("failed to list stacks: %w", err)
	}

	responses := make([]StackResponse, len(stacks))
	for i, stack := range stacks {
		responses[i] = h.toResponse(&stack)
	}

	return c.JSON(responses)
}

//	@Summary		Get a specific stack
//	@Description	Retrieve details of a specific stack by ID
//	@Tags			stacks
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Stack ID"
//	@Success		200	{object}	StackResponse
//	@Failure		400	{object}	fiberfx.ErrorResponse
//	@Failure		404	{object}	fiberfx.ErrorResponse
//	@Router			/stacks/{id} [get]
//
// Get a specific stack.
func (h *Handler) get(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	stack, err := h.stacksSvc.Get(c.Context(), id)
	if err != nil {
		return fmt.Errorf("failed to get stack: %w", err)
	}

	response := h.toResponse(stack)
	return c.JSON(response)
}

//	@Summary		Update a stack
//	@Description	Update an existing stack with the provided fields
//	@Tags			stacks
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string			true	"Stack ID"
//	@Param			stack	body		UpdateRequest	false	"Stack update request"
//	@Success		200		{object}	StackResponse
//	@Failure		400		{object}	fiberfx.ErrorResponse
//	@Failure		404		{object}	fiberfx.ErrorResponse
//	@Router			/stacks/{id} [patch]
//
// Update a stack.
func (h *Handler) patch(c *fiber.Ctx, req *UpdateRequest) error {
	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	updater := func(stack *stacks.Stack) error {
		if req.Description != nil {
			stack.Description = *req.Description
		}
		if req.GitURL != nil {
			stack.GitURL = *req.GitURL
		}
		if req.GitBranch != nil {
			stack.GitBranch = *req.GitBranch
		}
		if req.ComposePath != nil {
			stack.ComposePath = *req.ComposePath
		}
		if req.Variables != nil {
			stack.Variables = *req.Variables
		}
		if req.AutoDeploy != nil {
			stack.AutoDeploy = *req.AutoDeploy
		}
		if req.Labels != nil {
			stack.Labels = *req.Labels
		}
		return nil
	}

	err = h.stacksSvc.Update(c.Context(), id, updater)
	if err != nil {
		return fmt.Errorf("failed to update stack: %w", err)
	}

	return h.get(c)
}

//	@Summary		Delete a stack
//	@Description	Delete an existing stack by ID
//	@Tags			stacks
//	@Accept			json
//	@Produce		json
//	@Param			id	path	string	true	"Stack ID"
//	@Success		204
//	@Failure		400	{object}	fiberfx.ErrorResponse
//	@Failure		404	{object}	fiberfx.ErrorResponse
//	@Router			/stacks/{id} [delete]
//
// Delete a stack.
func (h *Handler) delete(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	err = h.stacksSvc.Delete(c.Context(), id)
	if err != nil {
		return fmt.Errorf("failed to delete stack: %w", err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) errorsHandler(c *fiber.Ctx) error {
	err := c.Next()
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, stacks.ErrNotFound), errors.Is(err, stacks.ErrNotAllowed):
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	case errors.Is(err, stacks.ErrConflict):
		return fiber.NewError(fiber.StatusConflict, err.Error())
	}

	return err //nolint:wrapcheck //alredy wrapped
}

func (h *Handler) toResponse(stack *stacks.Stack) StackResponse {
	return StackResponse{
		CreateRequest: CreateRequest{
			Name:        stack.Name,
			Description: stack.Description,
			GitURL:      stack.GitURL,
			GitBranch:   stack.GitBranch,
			ComposePath: stack.ComposePath,
			Variables:   stack.Variables,
			AutoDeploy:  stack.AutoDeploy,
			Labels:      stack.Labels,
		},
		ID: stack.ID,

		Status:     string(stack.Status),
		LastSync:   stack.LastSync,
		LastDeploy: stack.LastDeploy,
		CreatedAt:  stack.CreatedAt,
		UpdatedAt:  stack.UpdatedAt,
	}
}
