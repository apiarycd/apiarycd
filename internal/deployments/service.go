package deployments

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/apiarycd/apiarycd/internal/repositories"
	"github.com/apiarycd/apiarycd/internal/swarm"
	"github.com/apiarycd/apiarycd/internal/swarm/immutables"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

type Service struct {
	config Config

	deployments *Repository

	repositoriesSvc *repositories.Service
	swarmSvc        *swarm.Swarm
	renderer        *immutables.Renderer

	logger *zap.Logger

	// stackMu protects stackLocks map from concurrent access
	stackMu sync.Mutex
	// stackLocks holds per-stack mutexes to serialize deployments for the same stack
	stackLocks map[uuid.UUID]*sync.Mutex
}

func NewService(
	config Config,
	deployments *Repository,
	repositoriesSvc *repositories.Service,
	swarmSvc *swarm.Swarm,
	renderer *immutables.Renderer,
	logger *zap.Logger,
) *Service {
	return &Service{
		config: config,

		deployments: deployments,

		repositoriesSvc: repositoriesSvc,
		swarmSvc:        swarmSvc,
		renderer:        renderer,

		logger: logger,

		stackMu:    sync.Mutex{},
		stackLocks: make(map[uuid.UUID]*sync.Mutex),
	}
}

// create creates a new deployment.
func (s *Service) create(ctx context.Context, draft DeploymentDraft) (*Deployment, error) {
	s.logger.Info("creating deployment", zap.String("stack_id", draft.StackID.String()))

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

// LockStack locks the mutex for the given stack ID, creating it if necessary.
// The caller must call UnlockStack when done.
func (s *Service) LockStack(stackID uuid.UUID) {
	s.stackMu.Lock()
	mu, ok := s.stackLocks[stackID]
	if !ok {
		mu = &sync.Mutex{}
		s.stackLocks[stackID] = mu
	}
	s.stackMu.Unlock()

	mu.Lock()
}

// UnlockStack unlocks the mutex for the given stack ID.
// It must be called only after LockStack for the same stackID.
func (s *Service) UnlockStack(stackID uuid.UUID) {
	s.stackMu.Lock()
	mu := s.stackLocks[stackID]
	s.stackMu.Unlock()

	if mu == nil {
		return
	}

	mu.Unlock()
}

// Trigger triggers a deployment.
func (s *Service) Trigger(ctx context.Context, req DeploymentRequest) (*Deployment, error) {
	logger := s.logger.With(zap.String("stack_id", req.StackID.String()))

	logger.Info("triggering deployment")

	latest, err := s.deployments.GetLatestByStack(
		ctx,
		req.StackID,
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

	variables := maps.Clone(req.Variables)

	details, err := s.repositoriesSvc.GetDetails(ctx, req.StackID)
	if err != nil {
		logger.Error("failed to get repository details", zap.Error(err))
		return nil, fmt.Errorf("failed to get repository details: %w", err)
	}

	repositoryPath := details.Path

	// Update status to running and set started time
	now := time.Now()
	d, err := s.create(ctx, DeploymentDraft{
		StackID:            req.StackID,
		Version:            details.Commit,
		GitRef:             lo.CoalesceOrEmpty(details.Tag, details.Branch, details.Commit),
		Message:            details.CommitMessage,
		Variables:          variables,
		Status:             StatusRunning,
		StartedAt:          &now,
		PreviousDeployment: previousDeploymentID,
	})
	if err != nil {
		logger.Error("failed to create deployment", zap.Error(err))
		return nil, err
	}

	logger = logger.With(zap.String("deployment_id", d.ID.String()))

	logs, deployErr := s.deploy(ctx, repositoryPath, req, variables)

	now = time.Now()
	if deployErr != nil {
		err = s.update(ctx, d.ID, func(d *Deployment) error {
			d.MarkFailed(now, deployErr.Error())
			d.Logs = logs
			return nil
		})
		if err != nil {
			logger.Error("failed to mark deployment as failed", zap.Error(err))
			return nil, fmt.Errorf("failed to mark deployment as failed: %w", err)
		}

		logger.Error("deployment failed", zap.Error(deployErr))
		return nil, fmt.Errorf("failed to deploy stack: %w", deployErr)
	}

	err = s.update(ctx, d.ID, func(d *Deployment) error {
		d.MarkDeployedAt(now)
		d.Logs = logs
		return nil
	})
	if err != nil {
		logger.Error(
			"failed to update deployment status after trigger",
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to update deployment status: %w", err)
	}

	result, err := s.Get(ctx, d.ID)
	if err != nil {
		return nil, err
	}

	logger.Info("deployment triggered successfully")
	return result, nil
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

func (s *Service) DeleteByStack(ctx context.Context, stackID uuid.UUID, stackName string) error {
	ctx, cancel := context.WithTimeout(ctx, s.config.DeployTimeout)
	defer cancel()

	if err := s.swarmSvc.RemoveStack(ctx, stackName); err != nil {
		return fmt.Errorf("failed to remove docker stack: %w", err)
	}

	if err := s.deployments.DeleteByStack(ctx, stackID); err != nil {
		return fmt.Errorf("failed to delete deployments by stack: %w", err)
	}

	return nil
}

func (s *Service) deploy(
	ctx context.Context,
	repositoryPath string,
	req DeploymentRequest,
	variables map[string]string,
) ([]string, error) {
	// Apply deployment timeout
	ctx, cancel := context.WithTimeout(ctx, s.config.DeployTimeout)
	defer cancel()

	composePath := strings.TrimSpace(req.ComposePath)
	if composePath == "" {
		return nil, fmt.Errorf("%w: compose path is required", ErrValidationFailed)
	}

	cleanComposePath := filepath.Clean(composePath)
	if filepath.IsAbs(cleanComposePath) {
		return nil, fmt.Errorf("%w: compose path must be relative to repository root", ErrNotAllowed)
	}

	composePath = filepath.Join(repositoryPath, cleanComposePath)
	resolvedComposePath, err := filepath.EvalSymlinks(composePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve compose path: %w", err)
	}
	rel, relErr := filepath.Rel(repositoryPath, resolvedComposePath)
	if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return nil, fmt.Errorf("%w: compose path escapes repository root", ErrNotAllowed)
	}

	info, err := os.Stat(composePath)
	if err != nil {
		return nil, fmt.Errorf("compose file does not exist: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%w: compose path must point to a file", ErrValidationFailed)
	}

	composeForDeploy := composePath
	rotationLogs := []string(nil)
	if s.config.RotateImmutableResources {
		renderedPath, rotated, renderErr := s.renderer.RenderVersionedCompose(req.StackName, composePath)
		if renderErr != nil {
			return nil, fmt.Errorf("failed to render versioned compose for immutable resources: %w", renderErr)
		}
		defer func() {
			_ = os.Remove(renderedPath)
		}()
		composeForDeploy = renderedPath
		rotationLogs = rotated
	}

	composeForDeploy, err = filepath.Rel(repositoryPath, composeForDeploy)
	if err != nil {
		return nil, fmt.Errorf("failed to get relative compose path: %w", err)
	}

	env := flattenEnv(variables)
	logs, err := s.swarmSvc.DeployStack(ctx, swarm.DeployStackRequest{
		StackName:   req.StackName,
		ComposePath: composeForDeploy,
		WorkDir:     repositoryPath,
		Env:         env,
	})
	logs = append(logs, rotationLogs...)
	if err != nil {
		return logs, fmt.Errorf("failed to deploy stack: %w", err)
	}

	return logs, nil
}

func flattenEnv(vars map[string]string) []string {
	if len(vars) == 0 {
		return nil
	}

	keys := make([]string, 0, len(vars))
	for key := range vars {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	env := make([]string, 0, len(keys))
	for _, key := range keys {
		env = append(env, key+"="+vars[key])
	}

	return env
}
