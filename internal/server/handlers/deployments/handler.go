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

func NewHandler(deploymentsSvc *deployments.Service, validator *validator.Validate, logger *zap.Logger) handler.Handler {
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

	switch {
	case errors.Is(err, deployments.ErrNotFound):
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
