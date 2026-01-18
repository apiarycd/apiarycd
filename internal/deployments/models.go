package deployments

import (
	"time"

	"github.com/apiarycd/apiarycd/internal/storage"
	"github.com/google/uuid"
)

// deploymentModel represents a stack deployment instance.
type deploymentModel struct {
	storage.BaseEntity

	// References
	StackID uuid.UUID `json:"stack_id"`

	// Deployment Details
	Version string `json:"version"` // Git commit SHA or tag
	GitRef  string `json:"git_ref"` // Branch, tag, or commit
	Message string `json:"message"` // Git commit message

	// Deployment Configuration
	Variables map[string]string `json:"variables"` // Deployment-specific variables

	// Status
	Status      Status     `json:"status"`       // pending, running, success, failed, cancelled
	StartedAt   *time.Time `json:"started_at"`   // When deployment started
	CompletedAt *time.Time `json:"completed_at"` // When deployment completed/failed
	Error       string     `json:"error"`        // Error message if failed

	// Logs and Metrics
	Logs []string `json:"logs"` // Deployment logs

	// Rollback Information
	PreviousDeployment *uuid.UUID `json:"previous_deployment"` // Previous deployment ID for rollback
}

func newDeploymentModel(draft *DeploymentDraft) *deploymentModel {
	if draft == nil {
		return nil
	}

	now := time.Now()
	return &deploymentModel{
		BaseEntity: storage.BaseEntity{
			ID:        uuid.Must(uuid.NewV7()),
			CreatedAt: now,
			UpdatedAt: now,
		},
		StackID:            draft.StackID,
		Version:            draft.Version,
		GitRef:             draft.GitRef,
		Message:            draft.Message,
		Variables:          draft.Variables,
		Status:             draft.Status,
		StartedAt:          draft.StartedAt,
		CompletedAt:        draft.CompletedAt,
		Error:              draft.Error,
		Logs:               draft.Logs,
		PreviousDeployment: draft.PreviousDeployment,
	}
}

func newDeploymentUpdateModel(source *deploymentModel, draft *DeploymentDraft) *deploymentModel {
	updated := newDeploymentModel(draft)
	updated.ID = source.ID
	updated.CreatedAt = source.CreatedAt
	updated.UpdatedAt = time.Now()

	return updated
}

func newDeployment(model *deploymentModel) *Deployment {
	if model == nil {
		return nil
	}

	return &Deployment{
		DeploymentDraft: DeploymentDraft{
			StackID:            model.StackID,
			Version:            model.Version,
			GitRef:             model.GitRef,
			Message:            model.Message,
			Variables:          model.Variables,
			Status:             model.Status,
			StartedAt:          model.StartedAt,
			CompletedAt:        model.CompletedAt,
			Error:              model.Error,
			Logs:               model.Logs,
			PreviousDeployment: model.PreviousDeployment,
		},
		ID:        model.ID,
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
	}
}
