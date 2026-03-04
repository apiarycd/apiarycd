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
	"github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/transport"
	"github.com/google/uuid"
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

	// Create a unique staging directory for atomic clone
	stagingDir := dir + ".tmp-" + uuid.New().String()

	// Clean up staging directory on any error
	defer func() {
		if _, err := os.Stat(stagingDir); err == nil {
			_ = os.RemoveAll(stagingDir)
		}
	}()

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
		return fmt.Errorf("failed to build authentication: %w", err)
	}

	cloneOptions.Auth = auth

	// Clone into staging directory
	if _, cloneErr := git.PlainCloneContext(ctx, stagingDir, cloneOptions); cloneErr != nil {
		return fmt.Errorf("failed to clone repository: %w", cloneErr)
	}

	// Verify target directory doesn't already exist to avoid overwrites
	if _, statErr := os.Stat(dir); statErr == nil {
		return fmt.Errorf("%w: target directory %q already exists", ErrCloneFailed, dir)
	} else if !os.IsNotExist(statErr) {
		return fmt.Errorf("failed to check target directory: %w", statErr)
	}

	// Atomically rename staging directory to final destination
	if renameErr := os.Rename(stagingDir, dir); renameErr != nil {
		return fmt.Errorf("failed to promote staging directory: %w", renameErr)
	}

	// Remove staging directory from defer cleanup since it's now the final directory
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

	head, err := repo.Head()
	if err != nil {
		return fmt.Errorf("failed to get repository HEAD: %w", err)
	}

	pullReferenceName := head.Name()

	auth, err := s.buildAuth(req.Auth)
	if err != nil {
		return fmt.Errorf("failed to build authentication: %w", err)
	}

	if req.Branch != "" {
		targetReferenceName := plumbing.NewBranchReferenceName(req.Branch)
		pullReferenceName = targetReferenceName

		if head.Name() != targetReferenceName {
			if err = s.checkoutBranch(ctx, repo, worktree, req.Branch, auth); err != nil {
				return err
			}
		}
	}

	pullOptions := &git.PullOptions{
		Depth:         1,
		RemoteName:    "origin",
		ReferenceName: pullReferenceName,
		SingleBranch:  true,
		Auth:          auth,
	}

	if pullErr := worktree.PullContext(
		ctx,
		pullOptions,
	); pullErr != nil &&
		!errors.Is(pullErr, git.NoErrAlreadyUpToDate) {
		return fmt.Errorf("failed to pull repository: %w", pullErr)
	}

	return nil
}

func (s *Service) checkoutBranch(
	ctx context.Context,
	repo *git.Repository,
	worktree *git.Worktree,
	branch string,
	auth transport.AuthMethod,
) error {
	targetReferenceName := plumbing.NewBranchReferenceName(branch)
	if _, err := repo.Reference(targetReferenceName, true); err == nil {
		if checkoutErr := worktree.Checkout(&git.CheckoutOptions{Branch: targetReferenceName}); checkoutErr != nil {
			return fmt.Errorf("failed to checkout branch %q: %w", branch, checkoutErr)
		}

		return nil
	} else if !errors.Is(err, plumbing.ErrReferenceNotFound) {
		return fmt.Errorf("failed to lookup branch %q: %w", branch, err)
	}

	remoteReferenceName := plumbing.NewRemoteReferenceName("origin", branch)
	if fetchErr := repo.FetchContext(ctx, &git.FetchOptions{
		RemoteName: "origin",
		RefSpecs: []config.RefSpec{
			config.RefSpec(fmt.Sprintf("+refs/heads/%s:refs/remotes/origin/%s", branch, branch)),
		},
		Depth: 1,
		Auth:  auth,
	}); fetchErr != nil && !errors.Is(fetchErr, git.NoErrAlreadyUpToDate) {
		if errors.Is(fetchErr, git.ErrRemoteRefNotFound) {
			return fmt.Errorf("%w: %q", ErrBranchNotFound, branch)
		}

		return fmt.Errorf("failed to fetch branch %q: %w", branch, fetchErr)
	}

	remoteReference, err := repo.Reference(remoteReferenceName, true)
	if err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return fmt.Errorf("%w: %q", ErrBranchNotFound, branch)
		}

		return fmt.Errorf("failed to lookup remote branch %q: %w", branch, err)
	}

	if checkoutErr := worktree.Checkout(&git.CheckoutOptions{
		Branch: targetReferenceName,
		Hash:   remoteReference.Hash(),
		Create: true,
	}); checkoutErr != nil {
		return fmt.Errorf("failed to checkout branch %q: %w", branch, checkoutErr)
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
