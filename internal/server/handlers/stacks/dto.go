package stacks

import (
	"time"

	"github.com/apiarycd/apiarycd/internal/deployments"
	"github.com/google/uuid"
)

type GitAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// POSTRequest represents the request payload for creating a stack.
type POSTRequest struct {
	Name        string            `json:"name"                validate:"required,min=1,max=100"`
	Description string            `json:"description"         validate:"max=500"`
	GitURL      string            `json:"git_url"             validate:"required,url"`
	GitBranch   string            `json:"git_branch"          validate:"required,min=1,max=100"`
	GitAuth     GitAuth           `json:"git_auth,omitempty"`
	ComposePath string            `json:"compose_path"        validate:"required,min=1,max=255"`
	Variables   map[string]string `json:"variables,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

// PATCHRequest represents the request payload for updating a stack.
type PATCHRequest struct {
	Description *string            `json:"description,omitempty"  validate:"omitempty,max=500"`
	GitURL      *string            `json:"git_url,omitempty"      validate:"omitempty,url"`
	GitBranch   *string            `json:"git_branch,omitempty"   validate:"omitempty,min=1,max=100"`
	GitAuth     *GitAuth           `json:"git_auth"`
	ComposePath *string            `json:"compose_path,omitempty" validate:"omitempty,min=1,max=255"`
	Variables   *map[string]string `json:"variables,omitempty"`
	Labels      *map[string]string `json:"labels,omitempty"`
}

// StackResponse represents the response payload for a stack.
type StackResponse struct {
	POSTRequest

	ID         uuid.UUID  `json:"id"`
	Status     string     `json:"status"`
	LastSync   *time.Time `json:"last_sync,omitempty"`
	LastDeploy *time.Time `json:"last_deploy,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

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
	RollbackFrom *uuid.UUID `json:"rollback_from"` // Previous deployment ID for rollback

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func newDeploymentResponse(domain *deployments.Deployment) DeploymentResponse {
	return DeploymentResponse{
		ID:           domain.ID,
		StackID:      domain.StackID,
		Version:      domain.Version,
		GitRef:       domain.GitRef,
		Message:      domain.Message,
		Variables:    domain.Variables,
		Status:       domain.Status,
		StartedAt:    domain.StartedAt,
		CompletedAt:  domain.CompletedAt,
		Error:        domain.Error,
		Logs:         domain.Logs,
		RollbackFrom: domain.RollbackFrom,
		CreatedAt:    domain.CreatedAt,
		UpdatedAt:    domain.UpdatedAt,
	}
}
