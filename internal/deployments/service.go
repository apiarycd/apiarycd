package deployments

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"time"

	"github.com/apiarycd/apiarycd/internal/stacks"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Service struct {
	deployments *Repository

	stacksSvc *stacks.Service

	logger *zap.Logger
}

func NewService(deployments *Repository, stacksSvc *stacks.Service, logger *zap.Logger) *Service {
	return &Service{
		deployments: deployments,

		stacksSvc: stacksSvc,

		logger: logger,
	}
}

// create creates a new deployment.
func (s *Service) create(ctx context.Context, draft DeploymentDraft) (*Deployment, error) {
	s.logger.Info("creating deployment", zap.String("stack_id", draft.StackID.String()))

	if _, err := s.stacksSvc.Get(ctx, draft.StackID); err != nil {
		s.logger.Error("failed to get stack", zap.Error(err))
		return nil, fmt.Errorf("failed to get stack: %w", err)
	}

	deployment, err := s.deployments.Create(ctx, &draft)
	if err != nil {
		s.logger.Error("failed to create deployment", zap.Error(err))
		return nil, err
	}

	s.logger.Info("deployment created", zap.String("id", deployment.ID.String()))
	return deployment, nil
}

// Get retrieves a deployment by ID.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Deployment, error) {
	s.logger.Debug("getting deployment", zap.String("id", id.String()))

	deployment, err := s.deployments.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("failed to get deployment", zap.String("id", id.String()), zap.Error(err))
		return nil, err
	}

	return deployment, nil
}

// ListByStack retrieves all deployments.
func (s *Service) ListByStack(ctx context.Context, stackID uuid.UUID) ([]Deployment, error) {
	s.logger.Debug("listing deployments")

	deployments, err := s.deployments.ListByStack(ctx, stackID)
	if err != nil {
		s.logger.Error("failed to list deployments", zap.Error(err))
		return nil, err
	}

	return deployments, nil
}

// update updates an existing deployment.
func (s *Service) update(ctx context.Context, id uuid.UUID, updater func(*Deployment) error) error {
	s.logger.Info("updating deployment", zap.String("id", id.String()))

	err := s.deployments.Update(ctx, id, func(deployment *Deployment) error {
		if err := updater(deployment); err != nil {
			return err
		}
		deployment.UpdatedAt = time.Now()
		return nil
	})
	if err != nil {
		s.logger.Error("failed to update deployment", zap.String("id", id.String()), zap.Error(err))
		return err
	}

	s.logger.Info("deployment updated", zap.String("id", id.String()))
	return nil
}

// Trigger triggers a deployment (placeholder for deployment logic).
func (s *Service) Trigger(ctx context.Context, req DeploymentRequest) (*Deployment, error) {
	logger := s.logger.With(zap.String("stack_id", req.StackID.String()))

	logger.Info("triggering deployment")

	// Get the stack
	stack, err := s.stacksSvc.Get(ctx, req.StackID)
	if err != nil {
		logger.Error("failed to get stack for trigger", zap.Error(err))
		return nil, fmt.Errorf("failed to get stack for trigger: %w", err)
	}

	latest, err := s.deployments.GetLatestByStack(
		ctx,
		stack.ID,
		func(d *Deployment) bool { return d.Status == StatusSuccess },
	)
	if err != nil && !errors.Is(err, ErrNotFound) {
		logger.Error("failed to get latest deployment", zap.Error(err))
		return nil, err
	}

	var previousDeploymentID *uuid.UUID
	if latest != nil {
		previousDeploymentID = &latest.ID
	}

	variables := maps.Clone(stack.Variables)
	maps.Copy(variables, req.Variables)

	// TODO: Clone repository and determine git ref

	// Update status to running and set started time
	now := time.Now()
	d, err := s.create(ctx, DeploymentDraft{
		StackID:            stack.ID,
		Version:            "placeholder",
		GitRef:             "placeholder",
		Message:            "placeholder",
		Variables:          variables,
		Status:             StatusPending,
		StartedAt:          &now,
		CompletedAt:        nil,
		Error:              "",
		Logs:               []string{},
		PreviousDeployment: previousDeploymentID,
	})
	if err != nil {
		logger.Error("failed to create deployment", zap.Error(err))
		return nil, err
	}

	logger = logger.With(zap.String("deployment_id", d.ID.String()))

	// TODO: Implement actual deployment logic here (e.g., Docker Compose deployment)

	// For now, simulate deployment completion
	time.Sleep(time.Second) // Simulate some work

	now = time.Now()
	err = s.update(ctx, d.ID, func(d *Deployment) error {
		d.MarkDeployedAt(now)
		return nil
	})
	if err != nil {
		logger.Error(
			"failed to update deployment status after trigger",
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to update deployment status: %w", err)
	}

	logger.Info("deployment triggered successfully")
	return d, nil
}

func (s *Service) Rollback(ctx context.Context, stackID uuid.UUID) (*Deployment, *Deployment, error) {
	logger := s.logger.With(zap.String("stack_id", stackID.String()))

	latest, err := s.deployments.GetLatestByStack(
		ctx,
		stackID,
		func(d *Deployment) bool { return d.Status == StatusSuccess },
	)
	if err != nil {
		logger.Error("failed to get latest deployment", zap.Error(err))
		return nil, nil, err
	}

	logger = logger.With(zap.String("latest_deployment_id", latest.ID.String()))

	if latest.PreviousDeployment == nil {
		logger.Error("no previous deployment found")
		return nil, nil, fmt.Errorf("%w: no previous deployment found", ErrNotFound)
	}

	previous, err := s.deployments.GetByID(ctx, *latest.PreviousDeployment)
	if err != nil {
		logger.Error("failed to get previous deployment", zap.Error(err))
		return nil, nil, err
	}

	// TODO: Rollback deployment

	now := time.Now()
	if updErr := s.deployments.UpdateDual(
		ctx,
		latest.ID,
		previous.ID,
		func(d1, d2 *Deployment) error {
			d1.MarkRolledBack(now)
			d2.MarkDeployedAt(now)
			return nil
		},
	); updErr != nil {
		s.logger.Error("failed to update deployments", zap.Error(updErr))
		return nil, nil, updErr
	}

	return latest, previous, nil
}
