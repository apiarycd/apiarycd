package deployments

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusPending    Status = "pending"     // Deployment has not started
	StatusRunning    Status = "running"     // Deployment is in progress
	StatusSuccess    Status = "success"     // Deployment completed successfully
	StatusFailed     Status = "failed"      // Deployment failed
	StatusCancelled  Status = "cancelled"   // Deployment was cancelled
	StatusRolledBack Status = "rolled_back" // Deployment was rolled back
)

type DeploymentRequest struct {
	// References
	StackID uuid.UUID

	// Deployment Configuration
	Variables map[string]string // Deployment-specific variables
}

type DeploymentDraft struct {
	// References
	StackID uuid.UUID

	// Deployment Details
	Version string // Git commit SHA or tag
	GitRef  string // Branch, tag, or commit
	Message string // Git commit message

	// Deployment Configuration
	Variables map[string]string // Deployment-specific variables

	// Status
	Status      Status     // pending, running, success, failed, cancelled
	StartedAt   *time.Time // When deployment started
	CompletedAt *time.Time // When deployment completed/failed
	Error       string     // Error message if failed

	// Logs and Metrics
	Logs []string // Deployment logs

	// Rollback Information
	PreviousDeployment *uuid.UUID // Previous deployment ID for rollback
}

type Deployment struct {
	DeploymentDraft

	ID        uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (d *Deployment) MarkDeployedAt(deployedAt time.Time) {
	d.Status = StatusSuccess
	d.CompletedAt = &deployedAt
}

func (d *Deployment) MarkRolledBack(rolledBackAt time.Time) {
	d.Status = StatusRolledBack
	d.CompletedAt = &rolledBackAt
}
