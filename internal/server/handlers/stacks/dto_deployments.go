package stacks

import (
	"time"

	"github.com/apiarycd/apiarycd/internal/deployments"
	"github.com/google/uuid"
)

// POSTDeployRequest represents the request payload for deploying a stack.
type POSTDeployRequest struct {
	Variables map[string]string `json:"variables,omitempty"`
}

type DeploymentResponse struct {
	ID uuid.UUID `json:"id"`

	// References
	StackID uuid.UUID `json:"stack_id"`

	// Deployment Details
	Version string `json:"version"` // Git commit SHA or tag
	GitRef  string `json:"git_ref"` // Branch, tag, or commit
	Message string `json:"message"` // Git commit message

	// Deployment Configuration
	Variables map[string]string `json:"variables"` // Deployment-specific variables

	// Status
	Status      deployments.Status `json:"status"`       // pending, running, success, failed, cancelled
	StartedAt   *time.Time         `json:"started_at"`   // When deployment started
	CompletedAt *time.Time         `json:"completed_at"` // When deployment completed/failed
	Error       string             `json:"error"`        // Error message if failed

	// Logs and Metrics
	Logs []string `json:"logs"` // Deployment logs

	// Rollback Information
	PreviousDeployment *uuid.UUID `json:"previous_deployments"` // Previous deployment ID for rollback

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func newDeploymentResponse(domain *deployments.Deployment) DeploymentResponse {
	return DeploymentResponse{
		ID:                 domain.ID,
		StackID:            domain.StackID,
		Version:            domain.Version,
		GitRef:             domain.GitRef,
		Message:            domain.Message,
		Variables:          domain.Variables,
		Status:             domain.Status,
		StartedAt:          domain.StartedAt,
		CompletedAt:        domain.CompletedAt,
		Error:              domain.Error,
		Logs:               domain.Logs,
		PreviousDeployment: domain.PreviousDeployment,
		CreatedAt:          domain.CreatedAt,
		UpdatedAt:          domain.UpdatedAt,
	}
}
