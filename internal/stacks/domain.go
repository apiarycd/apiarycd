package stacks

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// GitAuthType represents the type of Git authentication.
type GitAuthType string

const (
	GitAuthTypeNone  GitAuthType = "none"
	GitAuthTypeHTTPS GitAuthType = "https"
	GitAuthTypeSSH   GitAuthType = "ssh"
)

// GitAuth represents Git repository authentication configuration.
type GitAuth struct {
	Type     GitAuthType `json:"type"`     // Authentication type: "none", "https", "ssh"
	Username string      `json:"username"` // Username for HTTPS auth
	Password string      `json:"password"` // Password or token for HTTPS auth

	// SSH-specific fields
	PrivateKeyPath string `json:"private_key_path,omitempty"` // Path to SSH private key
	Passphrase     string `json:"passphrase,omitempty"`       // Passphrase for encrypted SSH key
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

type StackUpdate struct {
	StackDraft

	// Status
	Status     Status     // active, inactive, error
	LastSync   *time.Time // Last successful sync
	LastDeploy *time.Time // Last successful deployment
}

type Stack struct {
	StackUpdate

	ID        uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
}

// GitService represents the interface for Git operations.
type GitService interface {
	// Clone clones a Git repository to the specified directory.
	Clone(ctx context.Context, url, branch, directory string, auth GitAuth) error

	// Pull pulls the latest changes for a Git repository.
	Pull(ctx context.Context, directory, branch string, auth GitAuth) error

	// RepositoryExists checks if a Git repository exists at the specified path.
	RepositoryExists(directory string) bool

	// RemoveRepository removes a Git repository from the filesystem.
	RemoveRepository(directory string) error

	// ValidateRepository validates that a repository URL is accessible.
	ValidateRepository(ctx context.Context, url string, auth GitAuth) error
}

// RepositoryPathBuilder builds repository paths for stacks.
type RepositoryPathBuilder interface {
	// BuildPath builds the repository path for a stack.
	BuildPath(stackID uuid.UUID) string
}
