package repositories

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/transport"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

var (
	ErrValidationFailed = errors.New("validation failed")
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

	return &Service{
		config: config,

		logger: logger,
	}, nil
}

func (s *Service) CloneOrPull(ctx context.Context, req CloneRequest) error {
	dir := s.buildPath(req.ID)

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

	dir := s.buildPath(req.ID)

	cloneOptions := &git.CloneOptions{
		URL:   req.URL,
		Depth: 1,
	}

	if req.Branch != "" {
		cloneOptions.SingleBranch = true
		cloneOptions.ReferenceName = plumbing.NewBranchReferenceName(req.Branch)
	}

	auth, err := s.buildAuth(req.Auth)
	if err != nil {
		return err
	}

	cloneOptions.Auth = auth

	if _, cloneErr := git.PlainCloneContext(ctx, dir, cloneOptions); cloneErr != nil {
		// Clean up partially cloned directory to avoid leaving corrupt state
		_ = os.RemoveAll(dir)
		return fmt.Errorf("failed to clone repository: %w", cloneErr)
	}

	return nil
}

func (s *Service) Pull(ctx context.Context, req PullRequest) error {
	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()

	if strings.TrimSpace(req.URL) == "" {
		return fmt.Errorf("%w: missing URL", ErrValidationFailed)
	}

	dir := s.buildPath(req.ID)

	repo, err := git.PlainOpen(dir)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	if valErr := validateRepositoryRemote(repo, req.URL); valErr != nil {
		return valErr
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	pullOptions := &git.PullOptions{
		Depth: 1,
	}

	if req.Branch != "" {
		pullOptions.SingleBranch = true
		pullOptions.ReferenceName = plumbing.NewBranchReferenceName(req.Branch)
	}

	auth, err := s.buildAuth(req.Auth)
	if err != nil {
		return err
	}

	pullOptions.Auth = auth

	if pullErr := worktree.PullContext(
		ctx,
		pullOptions,
	); pullErr != nil &&
		!errors.Is(pullErr, git.NoErrAlreadyUpToDate) {
		return fmt.Errorf("failed to pull repository: %w", pullErr)
	}

	return nil
}

func (s *Service) Delete(_ context.Context, id uuid.UUID) error {
	dir := s.buildPath(id)

	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("failed to remove repository: %w", err)
	}

	return nil
}

func (s *Service) buildAuth(auth GitAuth) (transport.AuthMethod, error) {
	am, ok, err := auth.BuildAuth()
	if err != nil {
		return nil, err
	}

	if ok {
		return am, nil
	}

	am, ok, err = s.config.Auth.BuildAuth()
	if err != nil {
		return nil, err
	}

	if !ok {
		s.logger.Warn("no authentication configured for repository operation")
	}

	return am, nil
}

func (s *Service) buildPath(id uuid.UUID) string {
	return filepath.Join(s.config.StorageDir, id.String())
}

func validateRepositoryRemote(repo *git.Repository, expectedURL string) error {
	remote, err := repo.Remote("origin")
	if err != nil {
		return fmt.Errorf("failed to get origin remote: %w", err)
	}

	urls := remote.Config().URLs
	if len(urls) == 0 {
		return fmt.Errorf("%w: origin remote has no configured URLs", ErrValidationFailed)
	}

	normalizedExpected := normalizeRemoteURL(expectedURL)
	for _, remoteURL := range urls {
		if normalizeRemoteURL(remoteURL) == normalizedExpected {
			return nil
		}
	}

	return fmt.Errorf("%w: origin remote URL does not match expected URL", ErrValidationFailed)
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
		return strings.ToLower(host + "/" + path)
	}

	if _, after, ok := strings.Cut(trimmed, "@"); ok {
		const hostPortParts = 2
		hostPath := after
		if parts := strings.SplitN(hostPath, ":", hostPortParts); len(parts) == hostPortParts {
			host := strings.ToLower(parts[0])
			path := strings.Trim(strings.TrimSuffix(parts[1], ".git"), "/")
			return strings.ToLower(host + "/" + path)
		}
	}

	return strings.ToLower(strings.Trim(strings.TrimSuffix(trimmed, ".git"), "/"))
}
