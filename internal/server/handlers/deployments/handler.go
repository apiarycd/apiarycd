package deployments

import (
	"errors"
	"fmt"

	"github.com/apiarycd/apiarycd/internal/deployments"
	"github.com/apiarycd/apiarycd/internal/server/validation"
	"github.com/go-core-fx/fiberfx/handler"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Handler struct {
	deploymentsSvc *deployments.Service

	validator *validator.Validate
	logger    *zap.Logger
}

func NewHandler(
	deploymentsSvc *deployments.Service,
	validator *validator.Validate,
	logger *zap.Logger,
) handler.Handler {
	return &Handler{
		deploymentsSvc: deploymentsSvc,

		validator: validator,
		logger:    logger,
	}
}

// Register implements handler.Handler.
func (h *Handler) Register(r fiber.Router) {
	r = r.Group("/deployments")

	r.Use(h.errorsHandler)
	r.Post("/", validation.DecorateWithBodyEx(h.validator, h.post))
	r.Get("/", h.list)
	r.Get("/:id", h.get)
	r.Patch("/:id", validation.DecorateWithBodyEx(h.validator, h.put))
	r.Delete("/:id", h.delete)
	r.Post("/:id/trigger", h.trigger)
}

//	@Summary		Create a new deployment
//	@Description	Create a new deployment for a stack with the provided configuration
//	@Tags			deployments
//	@Accept			json
//	@Produce		json
//	@Param			deployment	body		CreateRequest	true	"Deployment creation request"
//	@Success		201			{object}	DeploymentResponse
//	@Failure		400			{object}	fiberfx.ErrorResponse
//	@Failure		404			{object}	fiberfx.ErrorResponse
//	@Router			/deployments [post]
//
// Create a new deployment.
func (h *Handler) post(c *fiber.Ctx, req *CreateRequest) error {
	draft := deployments.DeploymentDraft{
		StackID:     req.StackID,
		Version:     req.Version,
		GitRef:      req.GitRef,
		Message:     req.Message,
		Variables:   req.Variables,
		Environment: req.Environment,
	}

	deployment, err := h.deploymentsSvc.Create(c.Context(), draft)
	if err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}

	response := h.toResponse(deployment)
	return c.Status(fiber.StatusCreated).JSON(response)
}

//	@Summary		List all deployments
//	@Description	Retrieve a list of all deployments
//	@Tags			deployments
//	@Accept			json
//	@Produce		json
//	@Success		200	{array}	DeploymentResponse
//	@Router			/deployments [get]
//
// List all deployments.
func (h *Handler) list(c *fiber.Ctx) error {
	deployments, err := h.deploymentsSvc.List(c.Context())
	if err != nil {
		return fmt.Errorf("failed to list deployments: %w", err)
	}

	responses := make([]DeploymentResponse, len(deployments))
	for i, deployment := range deployments {
		responses[i] = h.toResponse(&deployment)
	}

	return c.JSON(responses)
}

//	@Summary		Get a specific deployment
//	@Description	Retrieve details of a specific deployment by ID
//	@Tags			deployments
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Deployment ID"
//	@Success		200	{object}	DeploymentResponse
//	@Failure		400	{object}	fiberfx.ErrorResponse
//	@Failure		404	{object}	fiberfx.ErrorResponse
//	@Router			/deployments/{id} [get]
//
// Get a specific deployment.
func (h *Handler) get(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	deployment, err := h.deploymentsSvc.Get(c.Context(), id)
	if err != nil {
		return fmt.Errorf("failed to get deployment: %w", err)
	}

	response := h.toResponse(deployment)
	return c.JSON(response)
}

//	@Summary		Update a deployment
//	@Description	Update an existing deployment with the provided fields
//	@Tags			deployments
//	@Accept			json
//	@Produce		json
//	@Param			id			path		string			true	"Deployment ID"
//	@Param			deployment	body		UpdateRequest	false	"Deployment update request"
//	@Success		200			{object}	DeploymentResponse
//	@Failure		400			{object}	fiberfx.ErrorResponse
//	@Failure		404			{object}	fiberfx.ErrorResponse
//	@Router			/deployments/{id} [patch]
//
// Update a deployment.
func (h *Handler) put(c *fiber.Ctx, req *UpdateRequest) error {
	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	updater := func(deployment *deployments.Deployment) error {
		if req.Version != nil {
			deployment.Version = *req.Version
		}
		if req.GitRef != nil {
			deployment.GitRef = *req.GitRef
		}
		if req.Message != nil {
			deployment.Message = *req.Message
		}
		if req.Variables != nil {
			deployment.Variables = *req.Variables
		}
		if req.Environment != nil {
			deployment.Environment = *req.Environment
		}
		if req.Status != nil {
			deployment.Status = deployments.Status(*req.Status)
		}
		return nil
	}

	err = h.deploymentsSvc.Update(c.Context(), id, updater)
	if err != nil {
		return fmt.Errorf("failed to update deployment: %w", err)
	}

	return h.get(c)
}

//	@Summary		Delete a deployment
//	@Description	Delete an existing deployment by ID
//	@Tags			deployments
//	@Accept			json
//	@Produce		json
//	@Param			id	path	string	true	"Deployment ID"
//	@Success		204
//	@Failure		400	{object}	fiberfx.ErrorResponse
//	@Failure		404	{object}	fiberfx.ErrorResponse
//	@Router			/deployments/{id} [delete]
//
// Delete a deployment.
func (h *Handler) delete(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	err = h.deploymentsSvc.Delete(c.Context(), id)
	if err != nil {
		return fmt.Errorf("failed to delete deployment: %w", err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

//	@Summary		Trigger a deployment
//	@Description	Manually trigger the execution of a deployment
//	@Tags			deployments
//	@Accept			json
//	@Produce		json
//	@Param			id	path	string	true	"Deployment ID"
//	@Success		202
//	@Failure		400	{object}	fiberfx.ErrorResponse
//	@Failure		404	{object}	fiberfx.ErrorResponse
//	@Router			/deployments/{id}/trigger [post]
//
// Trigger a deployment.
func (h *Handler) trigger(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	err = h.deploymentsSvc.Trigger(c.Context(), id)
	if err != nil {
		return fmt.Errorf("failed to trigger deployment: %w", err)
	}

	return c.SendStatus(fiber.StatusAccepted)
}

func (h *Handler) errorsHandler(c *fiber.Ctx) error {
	err := c.Next()
	if err == nil {
		return nil
	}

	if errors.Is(err, deployments.ErrNotFound) {
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	}

	return err //nolint:wrapcheck //already wrapped
}

func (h *Handler) toResponse(deployment *deployments.Deployment) DeploymentResponse {
	return DeploymentResponse{
		CreateRequest: CreateRequest{
			StackID:     deployment.StackID,
			Version:     deployment.Version,
			GitRef:      deployment.GitRef,
			Message:     deployment.Message,
			Variables:   deployment.Variables,
			Environment: deployment.Environment,
		},

		ID:           deployment.ID,
		Status:       string(deployment.Status),
		StartedAt:    deployment.StartedAt,
		CompletedAt:  deployment.CompletedAt,
		Error:        deployment.Error,
		Logs:         deployment.Logs,
		HealthCheck:  deployment.HealthCheck,
		RollbackFrom: deployment.RollbackFrom,
		CreatedAt:    deployment.CreatedAt,
		UpdatedAt:    deployment.UpdatedAt,
	}
}
