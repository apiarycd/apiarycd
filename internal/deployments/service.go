package deployments

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/apiarycd/apiarycd/internal/repositories"
	"github.com/apiarycd/apiarycd/internal/stacks"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Service struct {
	config Config

	deployments *Repository

	stacksSvc       *stacks.Service
	repositoriesSvc *repositories.Service

	logger *zap.Logger

	// stackMu protects stackLocks map from concurrent access
	stackMu sync.Mutex
	// stackLocks holds per-stack mutexes to serialize deployments for the same stack
	stackLocks map[uuid.UUID]*sync.Mutex
}

func NewService(
	config Config,
	deployments *Repository,
	stacksSvc *stacks.Service,
	repositoriesSvc *repositories.Service,
	logger *zap.Logger,
) *Service {
	return &Service{
		config: config,

		deployments: deployments,

		stacksSvc:       stacksSvc,
		repositoriesSvc: repositoriesSvc,

		logger: logger,

		stackMu:    sync.Mutex{},
		stackLocks: make(map[uuid.UUID]*sync.Mutex),
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

// acquireStackLock returns the mutex for a given stack ID, creating it if necessary.
// The caller must call Unlock() on the returned mutex when done.
func (s *Service) acquireStackLock(stackID uuid.UUID) *sync.Mutex {
	s.stackMu.Lock()
	mu, ok := s.stackLocks[stackID]
	if !ok {
		mu = &sync.Mutex{}
		s.stackLocks[stackID] = mu
	}
	s.stackMu.Unlock()

	mu.Lock()
	return mu
}

// Trigger triggers a deployment.
func (s *Service) Trigger(ctx context.Context, req DeploymentRequest) (*Deployment, error) {
	logger := s.logger.With(zap.String("stack_id", req.StackID.String()))

	logger.Info("triggering deployment")

	// Acquire per-stack lock to serialize deployments for the same stack.
	// This prevents concurrent git worktree corruption and overlapping deploys.
	stackLock := s.acquireStackLock(req.StackID)
	defer stackLock.Unlock()

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
	if variables == nil {
		variables = make(map[string]string, len(req.Variables))
	}
	maps.Copy(variables, req.Variables)

	var repositoryPath string
	// Ensure local repository is up to date before deploying.
	if repositoryPath, err = s.stacksSvc.SyncRepository(ctx, stack.ID); err != nil {
		logger.Error("failed to synchronize stack repository", zap.Error(err))
		return nil, fmt.Errorf("failed to synchronize stack repository: %w", err)
	}

	// Update status to running and set started time
	now := time.Now()
	d, err := s.create(ctx, DeploymentDraft{
		StackID:            stack.ID,
		Version:            "latest",
		GitRef:             stack.GitBranch,
		Message:            "stack deploy",
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

	logs, deployErr := s.deploy(ctx, repositoryPath, *stack, variables)

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

func (s *Service) DeleteByStack(ctx context.Context, stackID uuid.UUID) error {
	// Acquire lock to wait for any in-flight deployment to complete
	stackLock := s.acquireStackLock(stackID)
	defer stackLock.Unlock()

	// Clean up the lock entry
	s.stackMu.Lock()
	delete(s.stackLocks, stackID)
	s.stackMu.Unlock()

	return s.deployments.DeleteByStack(ctx, stackID)
}

func (s *Service) deploy(
	ctx context.Context,
	repositoryPath string,
	stack stacks.Stack,
	variables map[string]string,
) ([]string, error) {
	// Apply deployment timeout
	ctx, cancel := context.WithTimeout(ctx, s.config.DeployTimeout)
	defer cancel()

	cleanComposePath := filepath.Clean(stack.ComposePath)
	if filepath.IsAbs(cleanComposePath) {
		return nil, fmt.Errorf("%w: compose path must be relative to repository root", ErrNotAllowed)
	}

	composePath := filepath.Join(repositoryPath, cleanComposePath)
	rel, relErr := filepath.Rel(repositoryPath, composePath)
	if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return nil, fmt.Errorf("%w: compose path escapes repository root", ErrNotAllowed)
	}

	if _, err := os.Stat(composePath); err != nil {
		return nil, fmt.Errorf("compose file does not exist: %w", err)
	}

	// Use minimal base environment to avoid leaking sensitive host variables
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
	}
	// Include DOCKER_* variables for Docker client configuration
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "DOCKER_") {
			env = append(env, e)
		}
	}
	env = append(env, flattenEnv(variables)...)
	args := []string{"stack", "deploy", "--compose-file", cleanComposePath, "--with-registry-auth", stack.Name}

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = repositoryPath
	cmd.Env = env

	output, err := cmd.CombinedOutput()
	logs := splitLines(output)
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

func splitLines(output []byte) []string {
	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" {
		return nil
	}

	lines := strings.Split(trimmed, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], "\r")
	}

	return lines
}
