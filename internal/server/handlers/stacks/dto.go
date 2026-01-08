package stacks

import (
	"time"

	"github.com/google/uuid"
)

type GitAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Stack struct {
	Name        string `json:"name"        validate:"required,min=1,max=100"`
	Description string `json:"description" validate:"max=500"`
	GitURL      string `json:"git_url"     validate:"required,url"`
	GitBranch   string `json:"git_branch"  validate:"required,min=1,max=100"`

	ComposePath string            `json:"compose_path"        validate:"required,min=1,max=255"`
	Variables   map[string]string `json:"variables,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

// POSTRequest represents the request payload for creating a stack.
type POSTRequest struct {
	Stack

	GitAuth GitAuth `json:"git_auth,omitempty"`
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
	Stack

	ID         uuid.UUID  `json:"id"`
	Status     string     `json:"status"`
	LastSync   *time.Time `json:"last_sync,omitempty"`
	LastDeploy *time.Time `json:"last_deploy,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}
