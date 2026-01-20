package git

import (
	"time"
)

// Authenticator represents an authentication method for Git operations.
type Authenticator interface {
	// GetAuth returns the go-git authentication object.
	GetAuth() interface{}
	// Type returns the authentication type.
	Type() string
}

// SSHAuth represents SSH key-based authentication.
type SSHAuth struct {
	PrivateKeyPath string // Path to private key file
	Passphrase     string // Passphrase for encrypted private key (optional)
}

// GetAuth returns the SSH authentication object for go-git.
func (a *SSHAuth) GetAuth() interface{} {
	return a // Will be handled in service with go-git's SSH auth
}

// Type returns "ssh".
func (a *SSHAuth) Type() string {
	return "ssh"
}

// HTTPSAuth represents HTTPS authentication.
type HTTPSAuth struct {
	Token    string // Personal access token (for GitHub, GitLab, etc.)
	Username string // Username for basic auth
	Password string // Password for basic auth
}

// GetAuth returns the HTTPS authentication object for go-git.
func (a *HTTPSAuth) GetAuth() interface{} {
	return a // Will be handled in service with go-git's basic auth
}

// Type returns "https".
func (a *HTTPSAuth) Type() string {
	return "https"
}

// CloneRequest represents the request to clone a repository.
type CloneRequest struct {
	URL       string        // Git repository URL
	Branch    string        // Branch to clone (optional, defaults to default branch)
	Directory string        // Directory to clone into
	Auth      Authenticator // Authentication method (optional)
}

// PullRequest represents the request to pull a repository.
type PullRequest struct {
	Path   string        // Path to the repository
	Branch string        // Branch to pull (optional)
	Auth   Authenticator // Authentication method (optional)
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
