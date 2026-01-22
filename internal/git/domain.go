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
	URL           string           // Git repository URL
	Branch        string           // Branch to clone (optional, defaults to default branch)
	Directory     string           // Directory to clone into
	Auth          Authenticator    // Authentication method (optional)
	Depth         *int             // Shallow clone depth (optional, nil for full clone)
	SingleBranch  bool             // Clone only the specified branch
	Progress      ProgressCallback // Progress callback (optional)
	Timeout       *time.Duration   // Operation timeout (optional)
	Validate      bool             // Validate repository after cloning
	RetryAttempts int              // Number of retry attempts on failure
}

// PullRequest represents the request to pull a repository.
type PullRequest struct {
	Path          string           // Path to the repository
	Branch        string           // Branch to pull (optional)
	Auth          Authenticator    // Authentication method (optional)
	Force         bool             // Force pull (discard local changes)
	Progress      ProgressCallback // Progress callback (optional)
	Timeout       *time.Duration   // Operation timeout (optional)
	RetryAttempts int              // Number of retry attempts on failure
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

// ProgressCallback is a callback function for reporting progress during Git operations.
type ProgressCallback func(progress string)

// BranchCreateRequest represents the request to create a new branch.
type BranchCreateRequest struct {
	Path       string // Path to the repository
	Name       string // New branch name
	BaseBranch string // Base branch to create from (optional, defaults to current)
	Checkout   bool   // Whether to checkout the new branch after creation
}

// BranchDeleteRequest represents the request to delete a branch.
type BranchDeleteRequest struct {
	Path  string // Path to the repository
	Name  string // Branch name to delete
	Force bool   // Force deletion even if not merged
}

// BranchSwitchRequest represents the request to switch/checkout a branch.
type BranchSwitchRequest struct {
	Path       string // Path to the repository
	Name       string // Branch name to switch to
	Create     bool   // Create branch if it doesn't exist
	BaseBranch string // Base branch when creating (optional)
}

// BranchMergeRequest represents the request to merge a branch.
type BranchMergeRequest struct {
	Path         string // Path to the repository
	SourceBranch string // Branch to merge from
	TargetBranch string // Branch to merge into (optional, defaults to current)
	FastForward  bool   // Allow fast-forward merge
	Message      string // Merge commit message (for non-fast-forward)
}

// TagCreateRequest represents the request to create a new tag.
type TagCreateRequest struct {
	Path       string // Path to the repository
	Name       string // Tag name
	Message    string // Tag message (for annotated tags, optional)
	CommitHash string // Commit hash to tag (optional, defaults to HEAD)
	Annotated  bool   // Create annotated tag (true) or lightweight (false)
	Sign       bool   // Sign the tag (requires GPG)
}

// TagDeleteRequest represents the request to delete a tag.
type TagDeleteRequest struct {
	Path string // Path to the repository
	Name string // Tag name to delete
}

// ReferenceFilter represents filtering options for branches and tags.
type ReferenceFilter struct {
	Pattern    string // Pattern to match names (glob pattern or regex)
	RemoteOnly bool   // Only remote references
	LocalOnly  bool   // Only local references
}

// ReferenceSort represents sorting options for branches and tags.
type ReferenceSort struct {
	By    string // Sort by: "name", "date", "commit"
	Order string // Sort order: "asc", "desc"
}

// BranchFilterRequest represents filtering and sorting options for branches.
type BranchFilterRequest struct {
	Path   string          // Path to the repository
	Filter ReferenceFilter // Filtering options
	Sort   ReferenceSort   // Sorting options
	Limit  int             // Maximum number of results (0 for unlimited)
	Offset int             // Skip first N results
}

// TagFilterRequest represents filtering and sorting options for tags.
type TagFilterRequest struct {
	Path   string          // Path to the repository
	Filter ReferenceFilter // Filtering options
	Sort   ReferenceSort   // Sorting options
	Limit  int             // Maximum number of results (0 for unlimited)
	Offset int             // Skip first N results
}

// BranchComparison represents the comparison between two branches.
type BranchComparison struct {
	Path            string   // Repository path
	BaseBranch      string   // Base branch name
	CompareBranch   string   // Branch to compare
	Ahead           int      // Number of commits ahead
	Behind          int      // Number of commits behind
	CommonAncestor  string   // Common ancestor commit hash
	DivergedCommits []string // List of commits that diverged
}

// TagComparison represents the comparison between two tags.
type TagComparison struct {
	Path           string // Repository path
	BaseTag        string // Base tag name
	CompareTag     string // Tag to compare
	Ahead          int    // Number of commits ahead
	Behind         int    // Number of commits behind
	CommonAncestor string // Common ancestor commit hash
}

// RemoteBranchInfo represents information about a remote branch.
type RemoteBranchInfo struct {
	Name      string // Branch name
	Remote    string // Remote name
	Hash      string // Commit hash
	IsTracked bool   // Whether this branch is being tracked locally
	Upstream  string // Upstream branch name (if tracked)
	Ahead     int    // Commits ahead of upstream
	Behind    int    // Commits behind upstream
}
