package git

import (
	"time"
)

// Repository represents a cloned Git repository.
type Repository struct {
	Path string // Path to the cloned repository
	URL  string // Original repository URL
}

// RepositoryStatus represents the status of a Git repository.
type RepositoryStatus struct {
	Path               string    // Repository path
	IsDirty            bool      // Has uncommitted changes
	CurrentBranch      string    // Current branch name
	RemoteURL          string    // Remote origin URL
	LastCommit         string    // Last commit hash
	LastCommitTime     time.Time // Last commit time
	UncommittedChanges []string  // List of uncommitted changes
}
