package stacks

import (
	"context"
	"fmt"
	"maps"

	"github.com/apiarycd/apiarycd/internal/deployments"
	"github.com/apiarycd/apiarycd/internal/repositories"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Service struct {
	stacks *Repository

	deploymentsSvc  *deployments.Service
	repositoriesSvc *repositories.Service

	logger *zap.Logger
}

func NewService(
	stacks *Repository,
	deploymentsSvc *deployments.Service,
	repositoriesSvc *repositories.Service,
	logger *zap.Logger,
) *Service {
	return &Service{
		stacks:          stacks,
		deploymentsSvc:  deploymentsSvc,
		repositoriesSvc: repositoriesSvc,
		logger:          logger,
	}
}

func (s *Service) Create(ctx context.Context, draft StackDraft) (*Stack, error) {
	s.logger.Info("creating stack", zap.String("name", draft.Name))

	stack, err := s.stacks.Create(ctx, draft)
	if err != nil {
		s.logger.Error("failed to create stack", zap.Error(err))
		return nil, err
	}

	if syncErr := s.repositoriesSvc.CloneOrPull(ctx, cloneRequestFromStack(*stack)); syncErr != nil {
		if deleteErr := s.stacks.Delete(ctx, stack.ID); deleteErr != nil {
			s.logger.Error("failed to rollback stack after clone error", zap.Error(deleteErr))
			return nil, fmt.Errorf(
				"failed to clone stack repository: %w; rollback delete failed: %w",
				syncErr,
				deleteErr,
			)
		}
		return nil, fmt.Errorf("failed to clone stack repository: %w", syncErr)
	}

	s.logger.Info("stack created", zap.String("id", stack.ID.String()))
	return stack, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Stack, error) {
	s.logger.Info("getting stack", zap.String("id", id.String()))

	stack, err := s.stacks.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("failed to get stack", zap.Error(err))
		return nil, err
	}

	return stack, nil
}

func (s *Service) List(ctx context.Context) ([]Stack, error) {
	s.logger.Info("listing stacks")

	stacks, err := s.stacks.List(ctx)
	if err != nil {
		s.logger.Error("failed to list stacks", zap.Error(err))
		return nil, err
	}

	s.logger.Info("stacks listed", zap.Int("count", len(stacks)))
	return stacks, nil
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, updater func(*Stack) error) error {
	s.logger.Info("updating stack", zap.String("id", id.String()))

	err := s.stacks.Update(ctx, id, func(stack *Stack) error {
		old := *stack

		if err := updater(stack); err != nil {
			return fmt.Errorf("failed to update stack: %w", err)
		}

		if syncErr := s.syncUpdatedStackRepository(ctx, &old, stack); syncErr != nil {
			return syncErr
		}

		return nil
	})
	if err != nil {
		s.logger.Error("failed to update stack", zap.Error(err))
		return err
	}

	s.logger.Info("stack updated", zap.String("id", id.String()))
	return nil
}

func (s *Service) SyncRepository(ctx context.Context, id uuid.UUID) (*Stack, error) {
	stack, err := s.stacks.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get stack before repository sync: %w", err)
	}

	if cloneErr := s.repositoriesSvc.CloneOrPull(ctx, cloneRequestFromStack(*stack)); cloneErr != nil {
		return nil, fmt.Errorf("failed to synchronize stack repository: %w", cloneErr)
	}

	return stack, nil
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	s.logger.Info("deleting stack", zap.String("id", id.String()))

	stack, err := s.stacks.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("failed to delete stack", zap.Error(err))
		return fmt.Errorf("failed to get stack for deletion: %w", err)
	}

	// Acquire per-stack lock to serialize deployments for the same stack.
	// This prevents concurrent git worktree corruption and overlapping deploys.
	s.deploymentsSvc.LockStack(id)
	defer s.deploymentsSvc.UnlockStack(id)

	if delErr := s.deploymentsSvc.DeleteByStack(ctx, stack.ID, stack.Name); delErr != nil {
		s.logger.Error("failed to delete stack deployments", zap.Error(delErr))
		return fmt.Errorf("failed to delete stack deployments: %w", delErr)
	}

	if delErr := s.stacks.Delete(ctx, id); delErr != nil {
		s.logger.Error("failed to delete stack", zap.Error(delErr))
		return fmt.Errorf("failed to delete stack: %w", delErr)
	}

	if delErr := s.repositoriesSvc.Delete(ctx, id); delErr != nil {
		s.logger.Error("failed to delete stack repository", zap.Error(delErr))
		return fmt.Errorf("failed to delete stack repository: %w", delErr)
	}

	s.logger.Info("stack deleted", zap.String("id", id.String()))
	return nil
}

func (s *Service) Deploy(
	ctx context.Context,
	id uuid.UUID,
	variables map[string]string,
) (*deployments.Deployment, error) {
	// Acquire per-stack lock to serialize deployments for the same stack.
	// This prevents concurrent git worktree corruption and overlapping deploys.
	s.deploymentsSvc.LockStack(id)
	defer s.deploymentsSvc.UnlockStack(id)

	stack, err := s.SyncRepository(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to sync repository before deployment: %w", err)
	}

	vars := make(map[string]string, len(stack.Variables)+len(variables))
	maps.Copy(vars, stack.Variables)
	maps.Copy(vars, variables)

	deployment, err := s.deploymentsSvc.Trigger(ctx, deployments.DeploymentRequest{
		StackID:     stack.ID,
		StackName:   stack.Name,
		ComposePath: stack.ComposePath,
		Variables:   vars,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to trigger deployment: %w", err)
	}

	return deployment, nil
}

func (s *Service) syncUpdatedStackRepository(ctx context.Context, current, next *Stack) error {
	req := cloneRequestFromStack(*next)

	if current.GitURL != next.GitURL || current.GitBranch != next.GitBranch {
		if err := s.repositoriesSvc.Clone(ctx, req); err != nil {
			return fmt.Errorf("failed to clone repository after URL or branch change: %w", err)
		}

		return nil
	}

	if current.GitAuth.Username != next.GitAuth.Username ||
		current.GitAuth.Password != next.GitAuth.Password {
		if err := s.repositoriesSvc.Pull(ctx, repositories.PullRequest(req)); err != nil {
			return fmt.Errorf("failed to pull repository after git settings change: %w", err)
		}
	}

	return nil
}

func cloneRequestFromStack(stack Stack) repositories.CloneRequest {
	var httpsAuth *repositories.GitHTTPSAuth
	if stack.GitAuth.Username != "" || stack.GitAuth.Password != "" {
		httpsAuth = &repositories.GitHTTPSAuth{
			Username: stack.GitAuth.Username,
			Password: stack.GitAuth.Password,
		}
	}

	return repositories.CloneRequest{
		ID:     stack.ID,
		URL:    stack.GitURL,
		Branch: stack.GitBranch,
		Auth: repositories.GitAuth{
			HTTPS: httpsAuth,
			SSH:   nil,
		},
	}
}
