package git

import (
	"time"
)

// CloneRequest represents the request to clone a repository.
type CloneRequest struct {
	URL       string // Git repository URL
	Branch    string // Branch to clone (optional, defaults to default branch)
	Directory string // Directory to clone into
}

// BranchInfo represents information about a Git branch.
type BranchInfo struct {
	Name      string // Branch name
	IsDefault bool   // Whether this is the default branch
	Hash      string // Latest commit hash on this branch
}

// TagInfo represents information about a Git tag.
type TagInfo struct {
	Name string    // Tag name
	Hash string    // Commit hash the tag points to
	Date time.Time // Tag creation date
}

// FileContent represents the content of a file.
type FileContent struct {
	Path    string // File path
	Content string // File content
}
