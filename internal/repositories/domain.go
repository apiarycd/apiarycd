package repositories

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v6/plumbing/transport"
	"github.com/go-git/go-git/v6/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v6/plumbing/transport/ssh"
	"github.com/google/uuid"
)

type GitAuth struct {
	HTTPS *GitHTTPSAuth
	SSH   *GitSSHAuth
}

func (a GitAuth) BuildAuth() (transport.AuthMethod, bool, error) {
	am, ok, err := a.SSH.BuildAuth()
	if err != nil {
		return nil, false, err
	}

	if ok {
		return am, ok, nil
	}

	am, ok = a.HTTPS.BuildAuth()

	return am, ok, nil
}

type GitHTTPSAuth struct {
	Username string
	Password string `json:"-"`
}

func (a *GitHTTPSAuth) BuildAuth() (transport.AuthMethod, bool) {
	if a == nil {
		return nil, false
	}

	if a.Username != "" && a.Password != "" {
		return &http.BasicAuth{
			Username: a.Username,
			Password: a.Password,
		}, true
	}

	if a.Password != "" {
		// Token-based auth (GitHub/GitLab PATs): username is arbitrary
		return &http.BasicAuth{
			Username: "git",
			Password: a.Password,
		}, true
	}

	return nil, false
}

type GitSSHAuth struct {
	PrivateKeyPath string
	Username       string
	Password       string `json:"-"`
}

func (c *GitSSHAuth) Validate() error {
	if c == nil {
		return nil
	}

	if c.PrivateKeyPath == "" {
		return nil
	}

	keyPath, err := expandHome(c.PrivateKeyPath)
	if err != nil {
		return err
	}

	if _, statErr := os.Stat(keyPath); statErr != nil {
		return fmt.Errorf("default private key path does not exist: %w", statErr)
	}

	return nil
}

func (c *GitSSHAuth) BuildAuth() (transport.AuthMethod, bool, error) {
	if c == nil {
		return nil, false, nil
	}

	if c.PrivateKeyPath == "" {
		return nil, false, nil
	}

	username := "git"
	if c.Username != "" {
		username = c.Username
	}

	keyPath, err := expandHome(c.PrivateKeyPath)
	if err != nil {
		return nil, false, err
	}

	keys, err := gitssh.NewPublicKeysFromFile(username, keyPath, c.Password)
	if err != nil {
		return nil, false, fmt.Errorf("failed to build SSH auth: %w", err)
	}

	return keys, true, nil
}

// CloneRequest represents the request to clone a repository.
type CloneRequest struct {
	ID     uuid.UUID // Repository ID
	URL    string    // Git repository URL
	Branch string    // Branch to clone (optional, defaults to default branch)
	Auth   GitAuth   // Authentication details
}

type PullRequest CloneRequest

func expandHome(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to resolve user home dir: %w", err)
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
	}
	return path, nil
}
