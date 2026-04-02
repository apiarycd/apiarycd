package repositories

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/samber/lo"
)

type GitAuth struct {
	HTTPS *GitHTTPSAuth
	SSH   *GitSSHAuth
}

type GitCommandAuth struct {
	Env         []string
	URLOverride string
}

func (a GitAuth) BuildCommandAuth(rawURL string) (*GitCommandAuth, error) {
	ca, err := a.SSH.BuildCommandAuth(rawURL)
	if err != nil {
		return nil, err
	}

	if ca != nil {
		return ca, nil
	}

	ca, err = a.HTTPS.BuildCommandAuth(rawURL)
	if err != nil {
		return nil, err
	}

	return ca, nil
}

type GitHTTPSAuth struct {
	Username string
	Password string `json:"-"`
}

func (a *GitHTTPSAuth) BuildCommandAuth(rawURL string) (*GitCommandAuth, error) {
	if a == nil || a.Password == "" {
		return nil, nil //nolint:nilnil // intentional
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse repository URL: %w", err)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, nil //nolint:nilnil // intentional
	}

	username := lo.CoalesceOrEmpty(
		parsed.User.Username(),
		a.Username,
		"git",
	)

	password, ok := parsed.User.Password()
	if !ok {
		password = a.Password
	}

	parsed.User = url.UserPassword(username, password)

	return &GitCommandAuth{Env: []string{}, URLOverride: parsed.String()}, nil
}

type GitSSHAuth struct {
	PrivateKeyPath string
}

func (a *GitSSHAuth) Validate() error {
	if a == nil {
		return nil
	}

	if a.PrivateKeyPath == "" {
		return nil
	}

	keyPath, err := expandHome(a.PrivateKeyPath)
	if err != nil {
		return err
	}

	if _, statErr := os.Stat(keyPath); statErr != nil {
		return fmt.Errorf("default private key path does not exist: %w", statErr)
	}

	return nil
}

func (a *GitSSHAuth) BuildCommandAuth(rawURL string) (*GitCommandAuth, error) {
	if a == nil || a.PrivateKeyPath == "" {
		return nil, nil //nolint:nilnil // intentional
	}

	if parsed, err := url.Parse(rawURL); err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") {
		return nil, nil //nolint:nilnil // intentional
	}

	keyPath, err := expandHome(a.PrivateKeyPath)
	if err != nil {
		return nil, err
	}

	sshCmd := fmt.Sprintf("ssh -o StrictHostKeyChecking=accept-new -o BatchMode=yes -i %s", shellEscapeArg(keyPath))

	return &GitCommandAuth{Env: []string{"GIT_SSH_COMMAND=" + sshCmd}, URLOverride: ""}, nil
}

// CloneRequest represents the request to clone a repository.
type CloneRequest struct {
	ID     uuid.UUID // Repository ID
	URL    string    // Git repository URL
	Branch string    // Branch to clone (optional, defaults to default branch)
	Auth   GitAuth   // Authentication details
}

type PullRequest CloneRequest

type Details struct {
	Path          string
	Branch        string
	Tag           string
	Commit        string
	CommitMessage string
}

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

func shellEscapeArg(input string) string {
	if input == "" {
		return "''"
	}

	return "'" + strings.ReplaceAll(input, "'", "'\\''") + "'"
}
