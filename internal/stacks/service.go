package stacks

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Service struct {
	stacks      *Repository
	gitSvc      GitService
	pathBuilder RepositoryPathBuilder

	logger *zap.Logger
}

func NewService(stacks *Repository, gitSvc GitService, pathBuilder RepositoryPathBuilder, logger *zap.Logger) *Service {
	return &Service{
		stacks:      stacks,
		gitSvc:      gitSvc,
		pathBuilder: pathBuilder,
		logger:      logger,
	}
}

func (s *Service) Create(ctx context.Context, draft StackDraft) (*Stack, error) {
	s.logger.Info("creating stack", zap.String("name", draft.Name))

	// Validate Git repository if URL is provided
	if draft.GitURL != "" {
		s.logger.Info("validating Git repository", zap.String("url", draft.GitURL))
		if err := s.gitSvc.ValidateRepository(ctx, draft.GitURL, draft.GitAuth); err != nil {
			s.logger.Error("Git repository validation failed", zap.Error(err))
			return nil, err
		}
	}

	stack, err := s.stacks.Create(ctx, draft)
	if err != nil {
		s.logger.Error("failed to create stack", zap.Error(err))
		return nil, err
	}

	// Clone Git repository if URL is provided
	if draft.GitURL != "" {
		repoPath := s.pathBuilder.BuildPath(stack.ID)
		s.logger.Info("cloning Git repository", zap.String("path", repoPath))

		if err := s.gitSvc.Clone(ctx, draft.GitURL, draft.GitBranch, repoPath, draft.GitAuth); err != nil {
			s.logger.Error("failed to clone Git repository", zap.Error(err))
			// Clean up the created stack if cloning fails
			if deleteErr := s.stacks.Delete(ctx, stack.ID); deleteErr != nil {
				s.logger.Error("failed to clean up stack after clone failure", zap.Error(deleteErr))
			}
			return nil, err
		}

		// Update stack with sync time
		now := time.Now()
		updateErr := s.stacks.Update(ctx, stack.ID, func(s *Stack) error {
			s.LastSync = &now
			return nil
		})
		if updateErr != nil {
			s.logger.Warn("failed to update stack sync time", zap.Error(updateErr))
		}
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

	// Get current stack state before update
	currentStack, err := s.stacks.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("failed to get current stack", zap.Error(err))
		return err
	}

	err = s.stacks.Update(ctx, id, updater)
	if err != nil {
		s.logger.Error("failed to update stack", zap.Error(err))
		return err
	}

	// Get updated stack state
	updatedStack, err := s.stacks.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("failed to get updated stack", zap.Error(err))
		return err
	}

	// Perform Git operations if Git configuration changed
	if s.hasGitConfigChanged(currentStack, updatedStack) && updatedStack.GitURL != "" {
		repoPath := s.pathBuilder.BuildPath(updatedStack.ID)

		// Check if repository exists
		if s.gitSvc.RepositoryExists(repoPath) {
			// Pull latest changes
			s.logger.Info("pulling Git repository", zap.String("path", repoPath))
			if err := s.gitSvc.Pull(ctx, repoPath, updatedStack.GitBranch, updatedStack.GitAuth); err != nil {
				s.logger.Error("failed to pull Git repository", zap.Error(err))
				// Set stack status to error
				s.stacks.Update(ctx, id, func(s *Stack) error {
					s.Status = StatusError
					return nil
				})
				return err
			}
		} else {
			// Clone repository if it doesn't exist
			s.logger.Info("cloning Git repository", zap.String("path", repoPath))
			if err := s.gitSvc.Clone(ctx, updatedStack.GitURL, updatedStack.GitBranch, repoPath, updatedStack.GitAuth); err != nil {
				s.logger.Error("failed to clone Git repository", zap.Error(err))
				// Set stack status to error
				s.stacks.Update(ctx, id, func(s *Stack) error {
					s.Status = StatusError
					return nil
				})
				return err
			}
		}

		// Update sync time and reset status
		now := time.Now()
		s.stacks.Update(ctx, id, func(s *Stack) error {
			s.LastSync = &now
			if s.Status == StatusError {
				s.Status = StatusActive
			}
			return nil
		})
	}

	s.logger.Info("stack updated", zap.String("id", id.String()))
	return nil
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	s.logger.Info("deleting stack", zap.String("id", id.String()))

	// Get stack before deletion to clean up repository
	stack, err := s.stacks.GetByID(ctx, id)
	if err != nil && err != ErrNotFound {
		s.logger.Error("failed to get stack for cleanup", zap.Error(err))
		// Continue with deletion even if we can't get the stack
	}

	// Clean up Git repository if it exists
	if stack != nil && stack.GitURL != "" {
		repoPath := s.pathBuilder.BuildPath(stack.ID)
		if s.gitSvc.RepositoryExists(repoPath) {
			s.logger.Info("removing Git repository", zap.String("path", repoPath))
			if err := s.gitSvc.RemoveRepository(repoPath); err != nil {
				s.logger.Warn("failed to remove Git repository", zap.Error(err))
				// Don't fail deletion if cleanup fails
			}
		}
	}

	err = s.stacks.Delete(ctx, id)
	if err != nil {
		s.logger.Error("failed to delete stack", zap.Error(err))
		return err
	}

	s.logger.Info("stack deleted", zap.String("id", id.String()))
	return nil
}

// hasGitConfigChanged checks if the Git configuration has changed between two stack versions.
func (s *Service) hasGitConfigChanged(old, new *Stack) bool {
	if old == nil || new == nil {
		return true
	}

	return old.GitURL != new.GitURL ||
		old.GitBranch != new.GitBranch ||
		old.GitAuth.Type != new.GitAuth.Type ||
		old.GitAuth.Username != new.GitAuth.Username ||
		old.GitAuth.Password != new.GitAuth.Password ||
		old.GitAuth.PrivateKeyPath != new.GitAuth.PrivateKeyPath ||
		old.GitAuth.Passphrase != new.GitAuth.Passphrase
}
