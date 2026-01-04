package swarm

import (
	"context"
	"fmt"

	"github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/client"
	"go.uber.org/zap"
)

// Swarm wraps Swarm-specific operations for the Docker client.
type Swarm struct {
	client *client.Client
	logger *zap.Logger
}

// NewSwarm creates a new Swarm wrapper.
func NewSwarm(client *client.Client, logger *zap.Logger) *Swarm {
	return &Swarm{
		client: client,
		logger: logger,
	}
}

// InspectSwarm inspects the current Swarm state.
func (s *Swarm) InspectSwarm(ctx context.Context) (swarm.Swarm, error) {
	s.logger.Debug("Inspecting Swarm")

	result, err := s.client.SwarmInspect(ctx, client.SwarmInspectOptions{})
	if err != nil {
		s.logger.Error("Failed to inspect Swarm", zap.Error(err))
		return swarm.Swarm{}, fmt.Errorf("failed to inspect Swarm: %w", err)
	}

	s.logger.Debug("Swarm inspection successful", zap.String("id", result.Swarm.ID))
	return result.Swarm, nil
}

// InitSwarm initializes a new Swarm.
func (s *Swarm) InitSwarm(ctx context.Context, req client.SwarmInitOptions) (string, error) {
	s.logger.Info("Initializing Swarm",
		zap.String("listenAddr", req.ListenAddr),
		zap.Bool("forceNewCluster", req.ForceNewCluster),
	)

	result, err := s.client.SwarmInit(ctx, req)
	if err != nil {
		s.logger.Error("Failed to initialize Swarm", zap.Error(err))
		return "", fmt.Errorf("failed to initialize Swarm: %w", err)
	}

	s.logger.Info("Swarm initialized successfully", zap.String("nodeID", result.NodeID))
	return result.NodeID, nil
}

// JoinSwarm joins an existing Swarm.
func (s *Swarm) JoinSwarm(ctx context.Context, req client.SwarmJoinOptions) error {
	s.logger.Info("Joining Swarm",
		zap.Strings("remoteAddrs", req.RemoteAddrs),
		zap.String("listenAddr", req.ListenAddr),
	)

	_, err := s.client.SwarmJoin(ctx, req)
	if err != nil {
		s.logger.Error("Failed to join Swarm", zap.Error(err))
		return fmt.Errorf("failed to join Swarm: %w", err)
	}

	s.logger.Info("Successfully joined Swarm")
	return nil
}

// LeaveSwarm leaves the current Swarm.
func (s *Swarm) LeaveSwarm(ctx context.Context, force bool) error {
	s.logger.Info("Leaving Swarm", zap.Bool("force", force))

	_, err := s.client.SwarmLeave(ctx, client.SwarmLeaveOptions{
		Force: force,
	})
	if err != nil {
		s.logger.Error("Failed to leave Swarm", zap.Error(err))
		return fmt.Errorf("failed to leave Swarm: %w", err)
	}

	s.logger.Info("Successfully left Swarm")
	return nil
}

// ListServices lists all services in the Swarm.
func (s *Swarm) ListServices(ctx context.Context) ([]swarm.Service, error) {
	s.logger.Debug("Listing Swarm services")

	result, err := s.client.ServiceList(ctx, client.ServiceListOptions{})
	if err != nil {
		s.logger.Error("Failed to list services", zap.Error(err))
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	s.logger.Debug("Services listed successfully", zap.Int("count", len(result.Items)))
	return result.Items, nil
}

// CreateService creates a new service in the Swarm.
func (s *Swarm) CreateService(ctx context.Context, service swarm.ServiceSpec) (string, error) {
	s.logger.Info("Creating service",
		zap.String("name", service.Name),
		zap.String("image", service.TaskTemplate.ContainerSpec.Image),
	)

	result, err := s.client.ServiceCreate(ctx, client.ServiceCreateOptions{
		Spec: service,
	})
	if err != nil {
		s.logger.Error("Failed to create service", zap.Error(err), zap.String("name", service.Name))
		return "", fmt.Errorf("failed to create service: %w", err)
	}

	s.logger.Info("Service created successfully",
		zap.String("id", result.ID),
		zap.String("name", service.Name),
	)
	return result.ID, nil
}

// RemoveService removes a service from the Swarm.
func (s *Swarm) RemoveService(ctx context.Context, serviceID string) error {
	s.logger.Info("Removing service", zap.String("id", serviceID))

	_, err := s.client.ServiceRemove(ctx, serviceID, client.ServiceRemoveOptions{})
	if err != nil {
		s.logger.Error("Failed to remove service", zap.Error(err), zap.String("id", serviceID))
		return fmt.Errorf("failed to remove service: %w", err)
	}

	s.logger.Info("Service removed successfully", zap.String("id", serviceID))
	return nil
}
