package deployments

import (
	"context"
	"fmt"
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

func NewService(deployments *Repository, logger *zap.Logger) *Service {
	return &Service{
		deployments: deployments,
		logger:      logger,
	}
}

// Create creates a new deployment.
func (s *Service) Create(ctx context.Context, draft DeploymentDraft) (*Deployment, error) {
	s.logger.Info("creating deployment", zap.String("stack_id", draft.StackID.String()))

	if _, err := s.stacksSvc.Get(ctx, draft.StackID); err != nil {
		s.logger.Error("failed to get stack", zap.Error(err))
		return nil, fmt.Errorf("failed to get stack: %w", err)
	}

	now := time.Now()
	deployment := &Deployment{
		DeploymentDraft: draft,
		ID:              uuid.New(),
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	err := s.deployments.Create(ctx, &deployment.DeploymentDraft)
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

// List retrieves all deployments.
func (s *Service) List(ctx context.Context) ([]Deployment, error) {
	s.logger.Debug("listing deployments")

	deployments, err := s.deployments.List(ctx)
	if err != nil {
		s.logger.Error("failed to list deployments", zap.Error(err))
		return nil, err
	}

	return deployments, nil
}

// Update updates an existing deployment.
func (s *Service) Update(ctx context.Context, id uuid.UUID, updater func(*Deployment) error) error {
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

// Delete deletes a deployment.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	s.logger.Info("deleting deployment", zap.String("id", id.String()))

	err := s.deployments.Delete(ctx, id)
	if err != nil {
		s.logger.Error("failed to delete deployment", zap.String("id", id.String()), zap.Error(err))
		return err
	}

	s.logger.Info("deployment deleted", zap.String("id", id.String()))
	return nil
}

// Trigger triggers a deployment (placeholder for deployment logic).
func (s *Service) Trigger(ctx context.Context, id uuid.UUID) error {
	s.logger.Info("triggering deployment", zap.String("id", id.String()))

	// Get the deployment
	_, err := s.deployments.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("failed to get deployment for trigger", zap.String("id", id.String()), zap.Error(err))
		return err
	}

	// Update status to running and set started time
	now := time.Now()
	err = s.Update(ctx, id, func(d *Deployment) error {
		d.Status = StatusRunning
		d.StartedAt = &now
		return nil
	})
	if err != nil {
		s.logger.Error("failed to update deployment status for trigger", zap.String("id", id.String()), zap.Error(err))
		return err
	}

	// TODO: Implement actual deployment logic here (e.g., Docker Compose deployment)

	// For now, simulate deployment completion
	time.Sleep(100 * time.Millisecond) // Simulate some work

	completedAt := time.Now()
	err = s.Update(ctx, id, func(d *Deployment) error {
		d.Status = StatusSuccess
		d.CompletedAt = &completedAt
		return nil
	})
	if err != nil {
		s.logger.Error("failed to update deployment status after trigger", zap.String("id", id.String()), zap.Error(err))
		return err
	}

	s.logger.Info("deployment triggered successfully", zap.String("id", id.String()))
	return nil
}
