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

// stackModel represents a GitOps stack configuration.
type stackModel struct {
	storage.BaseEntity

	// Basic Information
	Name        string `json:"name"`
	Description string `json:"description"`

	// Git Repository Information
	GitURL      string `json:"git_url"`      // HTTPS or SSH URL
	GitBranch   string `json:"git_branch"`   // Default branch to monitor
	ComposePath string `json:"compose_path"` // Path to docker-compose.yml

	// Configuration
	Variables  map[string]string `json:"variables"`   // Default variables
	AutoDeploy bool              `json:"auto_deploy"` // Auto-deploy on git push

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
		ComposePath: stack.ComposePath,
		Variables:   stack.Variables,
		AutoDeploy:  stack.AutoDeploy,
		Status:      stack.Status,
		LastSync:    stack.LastSync,
		LastDeploy:  stack.LastDeploy,
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
			ComposePath: model.ComposePath,
			Variables:   model.Variables,
			AutoDeploy:  model.AutoDeploy,
			Status:      model.Status,
			LastSync:    model.LastSync,
			LastDeploy:  model.LastDeploy,
			Labels:      model.Labels,
		},
		ID:        model.ID,
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
	}
}
