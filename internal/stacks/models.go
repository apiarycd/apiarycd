package stacks

import (
	"time"

	"github.com/apiarycd/apiarycd/internal/storage"
	"github.com/google/uuid"
)

type Status string

const (
	StatusActive   Status = "active"
	StatusInactive Status = "inactive"
	StatusError    Status = "error"
)

type gitAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// stackModel represents a GitOps stack configuration.
type stackModel struct {
	storage.BaseEntity

	// Basic Information
	Name        string `json:"name"`
	Description string `json:"description"`

	// Git Repository Information
	GitURL      string  `json:"git_url"`      // HTTPS or SSH URL
	GitBranch   string  `json:"git_branch"`   // Default branch to monitor
	GitAuth     gitAuth `json:"git_auth"`     // Git authentication
	ComposePath string  `json:"compose_path"` // Path to docker-compose.yml

	// Configuration
	Variables map[string]string `json:"variables"` // Default variables

	// Status
	Status     Status     `json:"status"`      // active, inactive, error
	LastSync   *time.Time `json:"last_sync"`   // Last successful sync
	LastDeploy *time.Time `json:"last_deploy"` // Last successful deployment

	// Metadata
	Labels map[string]string `json:"labels"` // Custom labels for filtering
}

func newStackModel(stack *StackDraft) *stackModel {
	if stack == nil {
		return nil
	}

	return &stackModel{
		BaseEntity: storage.BaseEntity{
			ID:        uuid.New(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Name:        stack.Name,
		Description: stack.Description,
		GitURL:      stack.GitURL,
		GitBranch:   stack.GitBranch,
		GitAuth: gitAuth{
			Username: stack.GitAuth.Username,
			Password: stack.GitAuth.Password,
		},
		ComposePath: stack.ComposePath,
		Variables:   stack.Variables,
		Status:      StatusActive,
		LastSync:    nil,
		LastDeploy:  nil,
		Labels:      stack.Labels,
	}
}

func newStack(model *stackModel) *Stack {
	if model == nil {
		return nil
	}

	return &Stack{
		StackDraft: StackDraft{
			Name:        model.Name,
			Description: model.Description,
			GitURL:      model.GitURL,
			GitBranch:   model.GitBranch,
			GitAuth: GitAuth{
				Username: model.GitAuth.Username,
				Password: model.GitAuth.Password,
			},
			ComposePath: model.ComposePath,
			Variables:   model.Variables,
			Labels:      model.Labels,
		},
		ID:        model.ID,
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,

		Status:     model.Status,
		LastSync:   model.LastSync,
		LastDeploy: model.LastDeploy,
	}
}
