package deployments

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusSuccess   Status = "success"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

type DeploymentDraft struct {
	// References
	StackID uuid.UUID

	// Deployment Details
	Version string // Git commit SHA or tag
	GitRef  string // Branch, tag, or commit
	Message string // Git commit message

	// Deployment Configuration
	Variables   map[string]string // Deployment-specific variables
	Environment string            // Environment name (prod, staging, etc.)

	// Status
	Status      Status     // pending, running, success, failed, cancelled
	StartedAt   *time.Time // When deployment started
	CompletedAt *time.Time // When deployment completed/failed
	Error       string     // Error message if failed

	// Logs and Metrics
	Logs        []string // Deployment logs
	HealthCheck string   // Health check URL or command

	// Rollback Information
	RollbackFrom *uuid.UUID // Previous deployment ID for rollback
}

type Deployment struct {
	DeploymentDraft

	ID        uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
}
