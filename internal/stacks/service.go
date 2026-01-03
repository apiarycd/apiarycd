package stacks

import "go.uber.org/zap"

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
