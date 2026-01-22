package stacks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/apiarycd/apiarycd/internal/git"
)

// gitAdapter adapts the git.Service to implement the stacks.GitService interface.
type gitAdapter struct {
	gitSvc *git.Service
}

// NewGitAdapter creates a new Git adapter.
func NewGitAdapter(gitSvc *git.Service) GitService {
	return &gitAdapter{gitSvc: gitSvc}
}

// Clone clones a Git repository to the specified directory.
func (a *gitAdapter) Clone(ctx context.Context, url, branch, directory string, auth GitAuth) error {
	// Convert stacks.GitAuth to git.Authenticator
	authenticator := a.convertAuth(auth)

	req := git.CloneRequest{
		URL:       url,
		Branch:    branch,
		Directory: directory,
		Auth:      authenticator,
		Validate:  true,
	}

	_, err := a.gitSvc.Clone(ctx, req)
	return err
}

// Pull pulls the latest changes for a Git repository.
func (a *gitAdapter) Pull(ctx context.Context, directory, branch string, auth GitAuth) error {
	// Convert stacks.GitAuth to git.Authenticator
	authenticator := a.convertAuth(auth)

	req := git.PullRequest{
		Path:   directory,
		Branch: branch,
		Auth:   authenticator,
	}

	return a.gitSvc.Pull(ctx, req)
}

// RepositoryExists checks if a Git repository exists at the specified path.
func (a *gitAdapter) RepositoryExists(directory string) bool {
	// Check if .git directory exists
	gitDir := filepath.Join(directory, ".git")
	_, err := os.Stat(gitDir)
	return !os.IsNotExist(err)
}

// RemoveRepository removes a Git repository from the filesystem.
func (a *gitAdapter) RemoveRepository(directory string) error {
	return os.RemoveAll(directory)
}

// ValidateRepository validates that a repository URL is accessible.
func (a *gitAdapter) ValidateRepository(ctx context.Context, url string, auth GitAuth) error {
	// For validation, we'll attempt a shallow clone to a temporary directory
	tempDir, err := os.MkdirTemp("", "git-validation-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	authenticator := a.convertAuth(auth)

	req := git.CloneRequest{
		URL:       url,
		Directory: tempDir,
		Auth:      authenticator,
		Depth:     &[]int{1}[0], // Shallow clone with depth 1
		Validate:  true,
	}

	if _, err := a.gitSvc.Clone(ctx, req); err != nil {
		return fmt.Errorf("repository validation failed: %w", err)
	}

	return nil
}

// convertAuth converts stacks.GitAuth to git.Authenticator.
func (a *gitAdapter) convertAuth(auth GitAuth) git.Authenticator {
	switch auth.Type {
	case GitAuthTypeSSH:
		return &git.SSHAuth{
			PrivateKeyPath: auth.PrivateKeyPath,
			Passphrase:     auth.Passphrase,
		}
	case GitAuthTypeHTTPS:
		return &git.HTTPSAuth{
			Token:    auth.Password, // Assume password is token for HTTPS
			Username: auth.Username,
			Password: auth.Password,
		}
	default:
		return nil // No authentication
	}
}
