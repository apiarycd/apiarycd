package stacks

import (
	"errors"
	"fmt"

	"github.com/apiarycd/apiarycd/internal/deployments"
	"github.com/apiarycd/apiarycd/internal/stacks"
	"github.com/go-core-fx/fiberfx/handler"
	"github.com/go-core-fx/fiberfx/validation"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

type Handler struct {
	stacksSvc      *stacks.Service
	deploymentsSvc *deployments.Service

	validator *validator.Validate
	logger    *zap.Logger
}

func NewHandler(
	stacksSvc *stacks.Service,
	deploymentsSvc *deployments.Service,
	validator *validator.Validate,
	logger *zap.Logger,
) handler.Handler {
	return &Handler{
		stacksSvc:      stacksSvc,
		deploymentsSvc: deploymentsSvc,

		validator: validator,
		logger:    logger,
	}
}

// Register implements handler.Handler.
func (h *Handler) Register(r fiber.Router) {
	r = r.Group("/stacks")

	r.Use(h.errorsHandler)
	// GET    /api/v1/stacks                 # List stacks
	r.Get("/", h.list)
	// POST   /api/v1/stacks                 # Create stack
	r.Post("/", validation.DecorateWithBodyEx(h.validator, h.post))
	// GET    /api/v1/stacks/{id}           # Get stack details
	r.Get("/:id", h.get)
	// PATCH  /api/v1/stacks/{id}           # Update stack
	r.Patch("/:id", validation.DecorateWithBodyEx(h.validator, h.patch))
	// DELETE /api/v1/stacks/{id}           # Delete stack
	r.Delete("/:id", h.delete)

	// POST   /api/v1/stacks/{id}/deploy    # Deploy stack
	r.Post("/:id/deploy", validation.DecorateWithBodyEx(h.validator, h.deploy))
	// GET    /api/v1/stacks/{id}/history   # Deployment history
	r.Get("/:id/history", h.history)
	// POST   /api/v1/stacks/{id}/rollback  # Rollback to previous version
	r.Post("/:id/rollback", h.rollback)
}

//	@Summary		Create a new stack
//	@Description	Create a new Docker Swarm stack with the provided configuration
//	@Tags			stacks
//	@Accept			json
//	@Produce		json
//	@Param			stack	body		POSTRequest	true	"Stack creation request"
//	@Success		201		{object}	StackResponse
//	@Failure		400		{object}	fiberfx.ErrorResponse
//	@Failure		409		{object}	fiberfx.ErrorResponse
//	@Router			/stacks [post]
//
// Create a new stack.
func (h *Handler) post(c *fiber.Ctx, req *POSTRequest) error {
	draft := &stacks.StackDraft{
		Name:        req.Name,
		Description: req.Description,
		GitURL:      req.GitURL,
		GitBranch:   req.GitBranch,
		GitAuth: stacks.GitAuth{
			Username: req.GitAuth.Username,
			Password: req.GitAuth.Password,
		},
		ComposePath: req.ComposePath,
		Variables:   req.Variables,
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
	id, err := getStackID(c)
	if err != nil {
		return err
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
//	@Param			id		path	string			true	"Stack ID"
//	@Param			stack	body	PATCHRequest	false	"Stack update request"
//	@Success		204
//	@Failure		400	{object}	fiberfx.ErrorResponse
//	@Failure		404	{object}	fiberfx.ErrorResponse
//	@Router			/stacks/{id} [patch]
//
// Update a stack.
func (h *Handler) patch(c *fiber.Ctx, req *PATCHRequest) error {
	id, err := getStackID(c)
	if err != nil {
		return err
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
		if req.GitAuth != nil {
			stack.GitAuth = stacks.GitAuth{
				Username: req.GitAuth.Username,
				Password: req.GitAuth.Password,
			}
		}
		if req.ComposePath != nil {
			stack.ComposePath = *req.ComposePath
		}
		if req.Variables != nil {
			stack.Variables = *req.Variables
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

	return c.SendStatus(fiber.StatusNoContent)
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
	id, err := getStackID(c)
	if err != nil {
		return err
	}

	err = h.stacksSvc.Delete(c.Context(), id)
	if err != nil {
		return fmt.Errorf("failed to delete stack: %w", err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// Deployments API.

//	@Summary		Deploy a stack
//	@Description	Trigger a deployment of a stack
//	@Tags			stacks, deployments
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string				true	"Stack ID"
//	@Param			deploy	body		POSTDeployRequest	false	"Deployment request"
//	@Success		200		{object}	DeploymentResponse
//	@Failure		400		{object}	fiberfx.ErrorResponse
//	@Failure		404		{object}	fiberfx.ErrorResponse
//	@Router			/stacks/{id}/deploy [post]
//
// Deploy a stack.
func (h *Handler) deploy(c *fiber.Ctx, req *POSTDeployRequest) error {
	id, err := getStackID(c)
	if err != nil {
		return err
	}

	d, err := h.deploymentsSvc.Trigger(
		c.Context(),
		deployments.DeploymentRequest{
			StackID:   id,
			Variables: req.Variables,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to trigger deployment: %w", err)
	}

	return c.JSON(newDeploymentResponse(d))
}

//	@Summary		List deployments for a stack
//	@Description	List all deployments for a stack
//	@Tags			stacks, deployments
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Stack ID"
//	@Success		200	{object}	[]DeploymentResponse
//	@Failure		400	{object}	fiberfx.ErrorResponse
//	@Failure		404	{object}	fiberfx.ErrorResponse
//	@Router			/stacks/{id}/history [get]
//
// List deployments for a stack.
func (h *Handler) history(c *fiber.Ctx) error {
	id, err := getStackID(c)
	if err != nil {
		return err
	}

	deps, err := h.deploymentsSvc.ListByStack(c.Context(), id)
	if err != nil {
		return fmt.Errorf("failed to list deployments: %w", err)
	}

	return c.JSON(
		lo.Map(
			deps,
			func(d deployments.Deployment, _ int) DeploymentResponse {
				return newDeploymentResponse(&d)
			},
		),
	)
}

//	@Summary		Rollback a stack
//	@Description	Rollback a stack to a previous version
//	@Tags			stacks
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Stack ID"
//	@Success		200	{object}	StackResponse
//	@Failure		400	{object}	fiberfx.ErrorResponse
//	@Failure		404	{object}	fiberfx.ErrorResponse
//	@Router			/stacks/{id}/rollback [post]
//
// Rollback a stack.
func (h *Handler) rollback(c *fiber.Ctx) error {
	id, err := getStackID(c)
	if err != nil {
		return err
	}

	_, current, err := h.deploymentsSvc.Rollback(c.Context(), id)
	if err != nil {
		return fmt.Errorf("failed to rollback stack: %w", err)
	}

	return c.JSON(newDeploymentResponse(current))
}

func (h *Handler) errorsHandler(c *fiber.Ctx) error {
	err := c.Next()
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, stacks.ErrNotAllowed):
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	case errors.Is(err, stacks.ErrNotFound):
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	case errors.Is(err, stacks.ErrConflict):
		return fiber.NewError(fiber.StatusConflict, err.Error())
	}

	if errors.Is(err, deployments.ErrNotFound) {
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	}

	return err //nolint:wrapcheck //already wrapped
}

func (h *Handler) toResponse(stack *stacks.Stack) StackResponse {
	return StackResponse{
		Stack: Stack{
			Name:        stack.Name,
			Description: stack.Description,
			GitURL:      stack.GitURL,
			GitBranch:   stack.GitBranch,
			ComposePath: stack.ComposePath,
			Variables:   stack.Variables,
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
