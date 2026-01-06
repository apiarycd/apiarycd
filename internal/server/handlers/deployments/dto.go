package deployments

import (
	"time"

	"github.com/google/uuid"
)

// CreateRequest represents the request payload for creating a deployment.
type CreateRequest struct {
	StackID     uuid.UUID         `json:"stack_id"            validate:"required"`
	Version     string            `json:"version"             validate:"required,min=1,max=100"`
	GitRef      string            `json:"git_ref"             validate:"required,min=1,max=100"`
	Message     string            `json:"message"             validate:"max=500"`
	Variables   map[string]string `json:"variables,omitempty"`
	Environment string            `json:"environment"         validate:"required,min=1,max=50"`
}

// UpdateRequest represents the request payload for updating a deployment.
type UpdateRequest struct {
	Version     *string            `json:"version,omitempty"     validate:"omitempty,min=1,max=100"`
	GitRef      *string            `json:"git_ref,omitempty"     validate:"omitempty,min=1,max=100"`
	Message     *string            `json:"message,omitempty"     validate:"omitempty,max=500"`
	Variables   *map[string]string `json:"variables,omitempty"`
	Environment *string            `json:"environment,omitempty" validate:"omitempty,min=1,max=50"`
	Status      *string            `json:"status,omitempty"      validate:"omitempty,oneof=pending running success failed cancelled"`
}

// DeploymentResponse represents the response payload for a deployment.
type DeploymentResponse struct {
	CreateRequest

	ID           uuid.UUID  `json:"id"`
	Status       string     `json:"status"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	Error        string     `json:"error,omitempty"`
	Logs         []string   `json:"logs,omitempty"`
	HealthCheck  string     `json:"health_check,omitempty"`
	RollbackFrom *uuid.UUID `json:"rollback_from,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}
