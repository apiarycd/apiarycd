package repositories

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

type Service struct {
	config Config

	logger *zap.Logger
}

func NewService(config Config, logger *zap.Logger) (*Service, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(config.StorageDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create storage dir: %w", err)
	}

	return &Service{config: config, logger: logger}, nil
}

func (s *Service) CloneOrPull(ctx context.Context, req CloneRequest) error {
	dir := s.BuildPath(req.ID)

	_, err := os.Stat(dir)
	switch {
	case os.IsNotExist(err):
		return s.Clone(ctx, req)
	case err != nil:
		return fmt.Errorf("failed to check repository directory: %w", err)
	}

	return s.Pull(ctx, PullRequest(req))
}

func (s *Service) Clone(ctx context.Context, req CloneRequest) error {
	if strings.TrimSpace(req.URL) == "" {
		return fmt.Errorf("%w: missing URL", ErrValidationFailed)
	}

	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()

	dir := s.BuildPath(req.ID)
	stagingDir := dir + ".tmp-" + uuid.New().String()

	defer func() {
		if _, err := os.Stat(stagingDir); err == nil {
			_ = os.RemoveAll(stagingDir)
		}
	}()

	auth, err := s.buildCommandAuth(req.Auth, req.URL)
	if err != nil {
		return fmt.Errorf("failed to build authentication: %w", err)
	}

	cloneURL := req.URL
	if auth != nil && auth.URLOverride != "" {
		cloneURL = auth.URLOverride
	}

	args := []string{"clone", "--depth", "1"}
	if req.Branch != "" {
		args = append(args, "--single-branch", "--branch", req.Branch)
	}
	args = append(args, cloneURL, stagingDir)

	if _, runErr := s.runGitCommand(ctx, "", auth, args...); runErr != nil {
		return fmt.Errorf("failed to clone repository: %w", runErr)
	}

	if _, statErr := os.Stat(dir); statErr == nil {
		if rmErr := os.RemoveAll(dir); rmErr != nil {
			return fmt.Errorf("failed to remove existing target directory: %w", rmErr)
		}
	} else if !os.IsNotExist(statErr) {
		return fmt.Errorf("failed to check target directory: %w", statErr)
	}

	if renameErr := os.Rename(stagingDir, dir); renameErr != nil {
		return fmt.Errorf("failed to promote staging directory: %w", renameErr)
	}

	return nil
}

func (s *Service) Pull(ctx context.Context, req PullRequest) error {
	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()

	if strings.TrimSpace(req.URL) == "" {
		return fmt.Errorf("%w: missing URL", ErrValidationFailed)
	}

	dir := s.BuildPath(req.ID)

	if valErr := s.validateRepositoryRemote(ctx, dir, req.URL); valErr != nil {
		return valErr
	}

	auth, err := s.buildCommandAuth(req.Auth, req.URL)
	if err != nil {
		return fmt.Errorf("failed to build authentication: %w", err)
	}

	pullBranch := ""
	if req.Branch != "" {
		pullBranch = req.Branch
		if err = s.checkoutBranch(ctx, dir, req.Branch, auth); err != nil {
			return err
		}
	}

	args := []string{"pull", "--depth", "1", "--ff-only"}
	if pullBranch != "" {
		remote := "origin"
		if auth != nil && auth.URLOverride != "" {
			remote = auth.URLOverride
		}
		args = append(args, remote, pullBranch)
	}

	if _, err = s.runGitCommand(ctx, dir, auth, args...); err != nil {
		if isBranchNotFoundError(err) {
			return fmt.Errorf("%w: failed to pull repository", ErrBranchNotFound)
		}
		return fmt.Errorf("failed to pull repository: %w", err)
	}

	return nil
}

func (s *Service) GetDetails(ctx context.Context, id uuid.UUID) (*Details, error) {
	dir := s.BuildPath(id)

	commitHash, err := s.runGitCommand(ctx, dir, nil, "rev-parse", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD: %w", err)
	}

	commitMessage, err := s.runGitCommand(ctx, dir, nil, "log", "-1", "--pretty=%B")
	if err != nil {
		return nil, fmt.Errorf("failed to get commit message: %w", err)
	}

	branch, err := s.runGitCommand(ctx, dir, nil, "symbolic-ref", "--short", "-q", "HEAD")
	if err != nil {
		branch = ""
	}

	tag, err := s.runGitCommand(ctx, dir, nil, "describe", "--tags", "--exact-match", "HEAD")
	if err != nil {
		tag = ""
	}

	return &Details{
		Path:          dir,
		Branch:        strings.TrimSpace(branch),
		Tag:           strings.TrimSpace(tag),
		Commit:        strings.TrimSpace(commitHash),
		CommitMessage: strings.TrimSpace(commitMessage),
	}, nil
}

func (s *Service) Delete(_ context.Context, id uuid.UUID) error {
	dir := s.BuildPath(id)

	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("failed to remove repository: %w", err)
	}

	return nil
}

func (s *Service) buildCommandAuth(auth GitAuth, rawURL string) (*GitCommandAuth, error) {
	am, err := auth.BuildCommandAuth(rawURL)
	if err != nil {
		return nil, err
	}

	if am != nil {
		return am, nil
	}

	am, err = s.config.Auth.BuildCommandAuth(rawURL)
	if err != nil {
		return nil, err
	}

	if am == nil {
		s.logger.Warn("no authentication configured for repository operation")
	}

	return am, nil
}

func (s *Service) BuildPath(id uuid.UUID) string {
	return filepath.Join(s.config.StorageDir, id.String())
}

func (s *Service) checkoutBranch(ctx context.Context, dir string, branch string, auth *GitCommandAuth) error {
	if _, err := s.runGitCommand(ctx, dir, auth, "show-ref", "--verify", "--quiet", "refs/heads/"+branch); err == nil {
		if _, checkoutErr := s.runGitCommand(ctx, dir, auth, "checkout", branch); checkoutErr != nil {
			return fmt.Errorf("failed to checkout branch %q: %w", branch, checkoutErr)
		}

		return nil
	}

	remote := "origin"
	if auth != nil && auth.URLOverride != "" {
		remote = auth.URLOverride
	}

	if _, fetchErr := s.runGitCommand(
		ctx,
		dir,
		auth,
		"fetch",
		"--depth",
		"1",
		remote,
		fmt.Sprintf("+refs/heads/%s:refs/remotes/origin/%s", branch, branch),
	); fetchErr != nil {
		if isBranchNotFoundError(fetchErr) {
			return fmt.Errorf("%w: %q", ErrBranchNotFound, branch)
		}
		return fmt.Errorf("failed to fetch branch %q: %w", branch, fetchErr)
	}

	if _, checkoutErr := s.runGitCommand(
		ctx,
		dir,
		auth,
		"checkout",
		"-B",
		branch,
		"refs/remotes/origin/"+branch,
	); checkoutErr != nil {
		return fmt.Errorf("failed to checkout branch %q: %w", branch, checkoutErr)
	}

	return nil
}

func (s *Service) validateRepositoryRemote(ctx context.Context, dir string, expectedURL string) error {
	remoteURL, err := s.runGitCommand(ctx, dir, nil, "remote", "get-url", "origin")
	if err != nil {
		return fmt.Errorf("failed to get origin remote: %w", err)
	}

	trimmed := strings.TrimSpace(remoteURL)
	if trimmed == "" {
		return fmt.Errorf("%w: origin remote has no configured URLs", ErrValidationFailed)
	}

	if normalizeRemoteURL(trimmed) != normalizeRemoteURL(expectedURL) {
		return fmt.Errorf("%w: origin remote URL does not match expected URL", ErrValidationFailed)
	}

	return nil
}

func (s *Service) runGitCommand(
	ctx context.Context,
	dir string,
	auth *GitCommandAuth,
	args ...string,
) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}

	env := append([]string{}, os.Environ()...)
	env = append(env, "GIT_TERMINAL_PROMPT=0")
	if auth != nil && len(auth.Env) > 0 {
		env = append(env, auth.Env...)
	}
	cmd.Env = env

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		subCommand := ""
		exitCode := 0
		if len(args) > 0 {
			subCommand = args[0]
		}
		if exitErr, ok := lo.ErrorsAs[*exec.ExitError](err); ok {
			exitCode = exitErr.ExitCode()
		}
		return "", &gitCommandError{
			ExitCode:   exitCode,
			SubCommand: subCommand,
			Err:        err,
			Stderr:     strings.TrimSpace(stderr.String()),
		}
	}

	return strings.TrimSpace(stdout.String()), nil
}

func normalizeRemoteURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	if parsed, err := url.Parse(trimmed); err == nil && parsed.Host != "" {
		host := strings.ToLower(parsed.Hostname())
		port := parsed.Port()
		if port != "" {
			host += ":" + port
		}
		path := strings.Trim(strings.TrimSuffix(parsed.Path, ".git"), "/")
		return host + "/" + path
	}

	if _, after, ok := strings.Cut(trimmed, "@"); ok {
		const hostPortParts = 2
		hostPath := after
		if parts := strings.SplitN(hostPath, ":", hostPortParts); len(parts) == hostPortParts {
			host := strings.ToLower(parts[0])
			path := strings.Trim(strings.TrimSuffix(parts[1], ".git"), "/")
			return host + "/" + path
		}
	}

	return strings.Trim(strings.TrimSuffix(trimmed, ".git"), "/")
}

// isBranchNotFoundError checks if the error from a git fetch command indicates
// that the remote branch does not exist. It looks for a gitCommandError with
// exit code 128 and the "couldn't find remote ref" message in Stderr.
func isBranchNotFoundError(err error) bool {
	gitErr, ok := lo.ErrorsAs[*gitCommandError](err)
	if !ok {
		return false
	}

	// Check if the stderr contains the expected message.
	return gitErr.ExitCode == 128 && strings.Contains(gitErr.Stderr, "couldn't find remote ref")
}
