package stacks

import (
	"time"

	"github.com/google/uuid"
)

type StackDraft struct {
	// Basic Information
	Name        string
	Description string

	// Git Repository Information
	GitURL      string // HTTPS or SSH URL
	GitBranch   string // Default branch to monitor
	ComposePath string // Path to docker-compose.yml

	// Configuration
	Variables  map[string]string // Default variables
	AutoDeploy bool              // Auto-deploy on git push

	// Status
	Status     Status     // active, inactive, error
	LastSync   *time.Time // Last successful sync
	LastDeploy *time.Time // Last successful deployment

	// Metadata
	Labels map[string]string // Custom labels for filtering
}

type Stack struct {
	StackDraft

	ID        uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
}
