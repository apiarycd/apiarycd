package stacks

import (
	"time"

	"github.com/google/uuid"
)

type GitAuth struct {
	Username string
	Password string
}

type StackDraft struct {
	// Basic Information
	Name        string
	Description string

	// Git Repository Information
	GitURL      string  // HTTPS or SSH URL
	GitBranch   string  // Default branch to monitor
	GitAuth     GitAuth // Authentication
	ComposePath string  // Path to docker-compose.yml

	// Configuration
	Variables map[string]string // Default variables

	// Metadata
	Labels map[string]string // Custom labels for filtering
}

type Stack struct {
	StackDraft

	ID        uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time

	// Status
	Status     Status     // active, inactive, error
	LastSync   *time.Time // Last successful sync
	LastDeploy *time.Time // Last successful deployment
}
