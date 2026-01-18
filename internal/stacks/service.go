package stacks

import (
	"context"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Service struct {
	stacks *Repository

	logger *zap.Logger
}

func NewService(stacks *Repository, logger *zap.Logger) *Service {
	return &Service{
		stacks: stacks,
		logger: logger,
	}
}

func (s *Service) Create(ctx context.Context, draft StackDraft) (*Stack, error) {
	s.logger.Info("creating stack", zap.String("name", draft.Name))

	stack, err := s.stacks.Create(ctx, draft)
	if err != nil {
		s.logger.Error("failed to create stack", zap.Error(err))
		return nil, err
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

	err := s.stacks.Update(ctx, id, updater)
	if err != nil {
		s.logger.Error("failed to update stack", zap.Error(err))
		return err
	}

	s.logger.Info("stack updated", zap.String("id", id.String()))
	return nil
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	s.logger.Info("deleting stack", zap.String("id", id.String()))

	err := s.stacks.Delete(ctx, id)
	if err != nil {
		s.logger.Error("failed to delete stack", zap.Error(err))
		return err
	}

	s.logger.Info("stack deleted", zap.String("id", id.String()))
	return nil
}
