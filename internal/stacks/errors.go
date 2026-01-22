package stacks

import "errors"

var (
	ErrNotFound   = errors.New("stack not found")
	ErrConflict   = errors.New("stack already exists")
	ErrNotAllowed = errors.New("operation not allowed")

	// Git-related errors
	ErrGitInvalidURL         = errors.New("invalid Git repository URL")
	ErrGitAuthFailed         = errors.New("Git authentication failed")
	ErrGitCloneFailed        = errors.New("failed to clone Git repository")
	ErrGitPullFailed         = errors.New("failed to pull Git repository")
	ErrGitRepositoryNotFound = errors.New("Git repository not found")
	ErrGitBranchNotFound     = errors.New("Git branch not found")
	ErrGitOperationTimeout   = errors.New("Git operation timed out")
)
