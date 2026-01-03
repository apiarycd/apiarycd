package deployments

import "go.uber.org/zap"

type Service struct {
	deployments *Repository

	logger *zap.Logger
}

func NewService(stacks *Repository, logger *zap.Logger) *Service {
	return &Service{
		deployments: stacks,
		logger:      logger,
	}
}
