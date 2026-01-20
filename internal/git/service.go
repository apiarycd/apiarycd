package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/go-git/go-git/v6/plumbing/transport"
	"github.com/go-git/go-git/v6/plumbing/transport/http"
	"github.com/go-git/go-git/v6/plumbing/transport/ssh"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"
)

type Service struct {
	config Config

	logger *zap.Logger

	// Concurrent operation limiter
	semaphore *semaphore.Weighted
}

// progressWriter implements io.Writer for progress callbacks.
type progressWriter struct {
	callback ProgressCallback
}

func (p *progressWriter) Write(data []byte) (int, error) {
	if p.callback != nil {
		p.callback(strings.TrimSpace(string(data)))
	}
	return len(data), nil
}

// NewService creates a new GitService.
func NewService(config Config, logger *zap.Logger) *Service {
	maxConcurrent := int64(config.Performance.MaxConcurrentOperations)
	if maxConcurrent <= 0 {
		maxConcurrent = 5 // default
	}

	return &Service{
		config:    config,
		logger:    logger,
		semaphore: semaphore.NewWeighted(maxConcurrent),
	}
}

// buildAuth converts an Authenticator to a go-git authentication object.
func (s *Service) buildAuth(auth Authenticator) (transport.AuthMethod, error) {
	if auth == nil {
		return nil, ErrAuthenticationFailed
	}

	switch a := auth.(type) {
	case *SSHAuth:
		return s.buildSSHAuth(a)
	case *HTTPSAuth:
		return s.buildHTTPSAuth(a)
	default:
		return nil, fmt.Errorf("unsupported authentication type: %s", auth.Type())
	}
}

// buildSSHAuth builds SSH authentication for go-git.
func (s *Service) buildSSHAuth(auth *SSHAuth) (*ssh.PublicKeys, error) {
	privateKeyPath := auth.PrivateKeyPath
	if privateKeyPath == "" {
		privateKeyPath = s.config.Auth.SSH.DefaultPrivateKey
	}

	if privateKeyPath == "" {
		return nil, fmt.Errorf("SSH private key path is required")
	}

	keys, err := ssh.NewPublicKeysFromFile("git", privateKeyPath, auth.Passphrase)
	if err != nil {
		return nil, fmt.Errorf("failed to load SSH private key: %w", err)
	}

	return keys, nil
}

// buildHTTPSAuth builds HTTPS authentication for go-git.
func (s *Service) buildHTTPSAuth(auth *HTTPSAuth) (*http.BasicAuth, error) {
	if auth.Token != "" {
		// For GitHub/GitLab tokens, use token as password, username can be anything
		return &http.BasicAuth{
			Username: "git", // or auth.Username if provided
			Password: auth.Token,
		}, nil
	}

	if auth.Username != "" && auth.Password != "" {
		return &http.BasicAuth{
			Username: auth.Username,
			Password: auth.Password,
		}, nil
	}

	// Try default token from config
	if s.config.Auth.HTTPS.DefaultToken != "" {
		return &http.BasicAuth{
			Username: s.config.Auth.HTTPS.DefaultUsername,
			Password: s.config.Auth.HTTPS.DefaultToken,
		}, nil
	}

	return nil, fmt.Errorf("HTTPS authentication requires token or username/password")
}

// Clone clones a repository to the specified directory.
func (s *Service) Clone(ctx context.Context, req CloneRequest) (*Repository, error) {
	s.logger.Info("cloning repository",
		zap.String("url", req.URL),
		zap.String("directory", req.Directory),
		zap.String("branch", req.Branch))

	// Check disk space before starting
	parentDir := filepath.Dir(req.Directory)
	if err := s.checkDiskSpace(ctx, parentDir); err != nil {
		return nil, err
	}

	// Acquire operation lock
	if err := s.acquireOperationLock(ctx); err != nil {
		return nil, fmt.Errorf("failed to acquire operation lock: %w", err)
	}
	defer s.releaseOperationLock()

	cloneOptions := &git.CloneOptions{
		URL:          req.URL,
		SingleBranch: req.SingleBranch,
	}

	if req.Depth != nil {
		cloneOptions.Depth = *req.Depth
	}

	if req.Branch != "" {
		cloneOptions.ReferenceName = plumbing.NewBranchReferenceName(req.Branch)
	}

	if req.Progress != nil {
		cloneOptions.Progress = &progressWriter{callback: req.Progress}
	}

	// Set up authentication
	if req.Auth != nil {
		auth, err := s.buildAuth(req.Auth)
		if err != nil {
			return nil, fmt.Errorf("failed to build authentication: %w", err)
		}
		cloneOptions.Auth = auth
	}

	// Check if directory already exists
	if _, statErr := os.Stat(req.Directory); statErr == nil {
		return nil, fmt.Errorf("%w: directory %s already exists", ErrRepositoryAlreadyExists, req.Directory)
	}

	// Apply timeout if specified
	if req.Timeout != nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *req.Timeout)
		defer cancel()
	}

	// Use default retry attempts if not specified
	retryAttempts := req.RetryAttempts
	if retryAttempts == 0 {
		retryAttempts = s.config.Performance.RetryAttempts
		if retryAttempts == 0 {
			retryAttempts = 3 // default
		}
	}

	// Retry logic
	var lastErr error
	for attempt := 0; attempt <= retryAttempts; attempt++ {
		if attempt > 0 {
			s.logger.Info("retrying clone", zap.Int("attempt", attempt), zap.Error(lastErr))
			time.Sleep(time.Duration(attempt) * time.Second)
		}

		_, err := git.PlainCloneContext(ctx, req.Directory, cloneOptions)
		if err != nil {
			lastErr = err
			if attempt == retryAttempts {
				s.logger.Error(
					"failed to clone repository after retries",
					zap.Error(err),
					zap.Int("attempts", retryAttempts+1),
				)
				return nil, fmt.Errorf("%w: %w", ErrCloneFailed, err)
			}
			continue
		}

		break
	}

	s.logger.Info("repository cloned successfully",
		zap.String("url", req.URL),
		zap.String("directory", req.Directory))

	repo := &Repository{
		Path: req.Directory,
		URL:  req.URL,
	}

	// Validate repository if requested
	if req.Validate {
		if err := s.ValidateRepository(ctx, req.Directory); err != nil {
			s.logger.Warn("repository validation failed after clone", zap.Error(err))
			// Don't fail the clone, just log
		}
	}

	return repo, nil
}

// Pull pulls the latest changes for the specified repository.
func (s *Service) Pull(ctx context.Context, req PullRequest) error {
	s.logger.Info("pulling repository",
		zap.String("path", req.Path),
		zap.String("branch", req.Branch),
		zap.Bool("force", req.Force))

	// Acquire operation lock
	if err := s.acquireOperationLock(ctx); err != nil {
		return fmt.Errorf("failed to acquire operation lock: %w", err)
	}
	defer s.releaseOperationLock()

	repo, err := git.PlainOpen(req.Path)
	if err != nil {
		s.logger.Error("failed to open repository", zap.Error(err))
		return fmt.Errorf("%w: %w", ErrRepositoryNotFound, err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		s.logger.Error("failed to get worktree", zap.Error(err))
		return fmt.Errorf("%w: %w", ErrInvalidRepository, err)
	}

	// Handle force pull - discard local changes
	if req.Force {
		status, statusErr := worktree.Status()
		if statusErr != nil {
			s.logger.Error("failed to get worktree status", zap.Error(statusErr))
			return fmt.Errorf("%w: %w", ErrInvalidRepository, statusErr)
		}

		if !status.IsClean() {
			s.logger.Warn("force pull: discarding local changes")
			statusErr = worktree.Reset(&git.ResetOptions{
				Mode: git.HardReset,
			})
			if statusErr != nil {
				s.logger.Error("failed to reset worktree", zap.Error(statusErr))
				return fmt.Errorf("failed to reset worktree for force pull: %w", statusErr)
			}
		}
	}

	pullOptions := &git.PullOptions{
		RemoteName:   "origin",
		SingleBranch: true,
		Depth:        1,
	}

	if req.Branch != "" {
		pullOptions.ReferenceName = plumbing.NewBranchReferenceName(req.Branch)
	}

	if req.Progress != nil {
		pullOptions.Progress = &progressWriter{callback: req.Progress}
	}

	// Set up authentication
	if req.Auth != nil {
		auth, authErr := s.buildAuth(req.Auth)
		if authErr != nil {
			return fmt.Errorf("failed to build authentication: %w", authErr)
		}
		pullOptions.Auth = auth
	}

	// Apply timeout if specified
	if req.Timeout != nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *req.Timeout)
		defer cancel()
	}

	// Use default retry attempts if not specified
	retryAttempts := req.RetryAttempts
	if retryAttempts == 0 {
		retryAttempts = s.config.Performance.RetryAttempts
		if retryAttempts == 0 {
			retryAttempts = 3 // default
		}
	}

	// Retry logic
	var lastErr error
	for attempt := 0; attempt <= retryAttempts; attempt++ {
		if attempt > 0 {
			s.logger.Info("retrying pull", zap.Int("attempt", attempt), zap.Error(lastErr))
			time.Sleep(time.Duration(attempt) * time.Second)
		}

		err = worktree.PullContext(ctx, pullOptions)
		if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
			lastErr = err
			if attempt == retryAttempts {
				s.logger.Error(
					"failed to pull repository after retries",
					zap.Error(err),
					zap.Int("attempts", retryAttempts+1),
				)
				return fmt.Errorf("%w: %w", ErrPullFailed, err)
			}
			continue
		}

		break
	}

	s.logger.Info("repository pulled successfully",
		zap.String("path", req.Path))

	return nil
}

// GetBranches retrieves all branches from the repository.
func (s *Service) GetBranches(_ context.Context, repoPath string) ([]BranchInfo, error) {
	s.logger.Info("getting branches",
		zap.String("path", repoPath))

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		s.logger.Error("failed to open repository", zap.Error(err))
		return nil, fmt.Errorf("%w: %w", ErrRepositoryNotFound, err)
	}

	branches, err := repo.Branches()
	if err != nil {
		s.logger.Error("failed to get branches", zap.Error(err))
		return nil, fmt.Errorf("%w: %w", ErrInvalidRepository, err)
	}

	var branchInfos []BranchInfo
	err = branches.ForEach(func(ref *plumbing.Reference) error {
		branchName := ref.Name().Short()

		// Get commit hash
		_, commitErr := repo.CommitObject(ref.Hash())
		if commitErr != nil {
			s.logger.Warn("failed to get commit for branch",
				zap.String("branch", branchName), zap.Error(commitErr))
			return nil // continue
		}

		// Check if it's the default branch (HEAD)
		head, commitErr := repo.Head()
		isDefault := commitErr == nil && head.Name() == ref.Name()

		branchInfos = append(branchInfos, BranchInfo{
			Name:      branchName,
			IsDefault: isDefault,
			Hash:      ref.Hash().String(),
		})

		return nil
	})

	if err != nil {
		s.logger.Error("failed to iterate branches", zap.Error(err))
		return nil, fmt.Errorf("%w: %w", ErrInvalidRepository, err)
	}

	s.logger.Info("branches retrieved",
		zap.String("path", repoPath),
		zap.Int("count", len(branchInfos)))

	return branchInfos, nil
}

// GetTags retrieves all tags from the repository.
func (s *Service) GetTags(_ context.Context, repoPath string) ([]TagInfo, error) {
	s.logger.Info("getting tags",
		zap.String("path", repoPath))

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		s.logger.Error("failed to open repository", zap.Error(err))
		return nil, fmt.Errorf("%w: %w", ErrRepositoryNotFound, err)
	}

	tags, err := repo.Tags()
	if err != nil {
		s.logger.Error("failed to get tags", zap.Error(err))
		return nil, fmt.Errorf("%w: %w", ErrInvalidRepository, err)
	}

	var tagInfos []TagInfo
	err = tags.ForEach(func(ref *plumbing.Reference) error {
		tagName := ref.Name().Short()

		// Get commit object
		obj, tagErr := repo.TagObject(ref.Hash())
		var commit *object.Commit
		var tagTime time.Time

		if tagErr == nil {
			// Annotated tag
			commit, tagErr = obj.Commit()
			if tagErr != nil {
				s.logger.Warn("failed to get commit for annotated tag",
					zap.String("tag", tagName), zap.Error(tagErr))
				return nil
			}
			tagTime = obj.Tagger.When
		} else {
			// Lightweight tag
			commit, tagErr = repo.CommitObject(ref.Hash())
			if tagErr != nil {
				s.logger.Warn("failed to get commit for lightweight tag",
					zap.String("tag", tagName), zap.Error(tagErr))
				return nil
			}
			tagTime = commit.Author.When
		}

		tagInfos = append(tagInfos, TagInfo{
			Name: tagName,
			Hash: commit.Hash.String(),
			Date: tagTime,
		})

		return nil
	})

	if err != nil {
		s.logger.Error("failed to iterate tags", zap.Error(err))
		return nil, fmt.Errorf("%w: %w", ErrInvalidRepository, err)
	}

	s.logger.Info("tags retrieved",
		zap.String("path", repoPath),
		zap.Int("count", len(tagInfos)))

	return tagInfos, nil
}

// GetFileContent retrieves the content of a file at the specified path.
func (s *Service) GetFileContent(_ context.Context, repoPath, filePath string) (string, error) {
	s.logger.Info("getting file content",
		zap.String("path", repoPath),
		zap.String("file", filePath))

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		s.logger.Error("failed to open repository", zap.Error(err))
		return "", fmt.Errorf("%w: %w", ErrRepositoryNotFound, err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		s.logger.Error("failed to get worktree", zap.Error(err))
		return "", fmt.Errorf("%w: %w", ErrInvalidRepository, err)
	}

	file, err := worktree.Filesystem.Open(filePath)
	if err != nil {
		s.logger.Error("failed to open file", zap.Error(err))
		return "", fmt.Errorf("%w: %w", ErrFileNotFound, err)
	}
	defer file.Close()

	content, err := os.ReadFile(filepath.Join(repoPath, filePath))
	if err != nil {
		s.logger.Error("failed to read file", zap.Error(err))
		return "", fmt.Errorf("%w: %w", ErrFileNotFound, err)
	}

	s.logger.Info("file content retrieved",
		zap.String("path", repoPath),
		zap.String("file", filePath),
		zap.Int("size", len(content)))

	return string(content), nil
}

// GetLatestCommit gets the latest commit SHA for the specified branch.
func (s *Service) GetLatestCommit(_ context.Context, repoPath, branch string) (string, error) {
	s.logger.Info("getting latest commit",
		zap.String("path", repoPath),
		zap.String("branch", branch))

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		s.logger.Error("failed to open repository", zap.Error(err))
		return "", fmt.Errorf("%w: %w", ErrRepositoryNotFound, err)
	}

	var refName plumbing.ReferenceName
	if branch == "" {
		head, headErr := repo.Head()
		if headErr != nil {
			s.logger.Error("failed to get HEAD", zap.Error(headErr))
			return "", fmt.Errorf("%w: %w", ErrInvalidRepository, headErr)
		}
		refName = head.Name()
	} else {
		refName = plumbing.NewBranchReferenceName(branch)
	}

	ref, err := repo.Reference(refName, true)
	if err != nil {
		s.logger.Error("failed to get reference", zap.Error(err))
		return "", fmt.Errorf("%w: %w", ErrBranchNotFound, err)
	}

	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		s.logger.Error("failed to get commit object", zap.Error(err))
		return "", fmt.Errorf("%w: %w", ErrInvalidRepository, err)
	}

	hash := commit.Hash.String()

	s.logger.Info("latest commit retrieved",
		zap.String("path", repoPath),
		zap.String("branch", branch),
		zap.String("hash", hash))

	return hash, nil
}

// ValidateRepository checks if a repository is valid and accessible.
func (s *Service) ValidateRepository(_ context.Context, repoPath string) error {
	s.logger.Info("validating repository", zap.String("path", repoPath))

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		s.logger.Error("failed to open repository", zap.Error(err))
		return fmt.Errorf("%w: %w", ErrRepositoryNotFound, err)
	}

	// Check if HEAD exists and is valid
	head, err := repo.Head()
	if err != nil {
		s.logger.Error("failed to get HEAD", zap.Error(err))
		return fmt.Errorf("%w: %w", ErrInvalidRepository, err)
	}

	// Try to get the commit object
	_, err = repo.CommitObject(head.Hash())
	if err != nil {
		s.logger.Error("failed to get HEAD commit", zap.Error(err))
		return fmt.Errorf("%w: %w", ErrInvalidRepository, err)
	}

	// Check remote configuration
	remotes, err := repo.Remotes()
	if err != nil {
		s.logger.Error("failed to get remotes", zap.Error(err))
		return fmt.Errorf("%w: %w", ErrInvalidRepository, err)
	}

	if len(remotes) == 0 {
		s.logger.Error("no remotes configured")
		return fmt.Errorf("%w: no remotes configured", ErrInvalidRepository)
	}

	s.logger.Info("repository validation successful", zap.String("path", repoPath))
	return nil
}

// CleanupRepository removes a local repository directory.
func (s *Service) CleanupRepository(ctx context.Context, repoPath string) error {
	s.logger.Info("cleaning up repository", zap.String("path", repoPath))

	// Check if it's actually a git repository before deleting
	if err := s.ValidateRepository(ctx, repoPath); err != nil {
		s.logger.Warn("repository validation failed during cleanup", zap.Error(err))
		// Still proceed with cleanup
	}

	err := os.RemoveAll(repoPath)
	if err != nil {
		s.logger.Error("failed to remove repository directory", zap.Error(err))
		return fmt.Errorf("failed to cleanup repository: %w", err)
	}

	s.logger.Info("repository cleanup successful", zap.String("path", repoPath))
	return nil
}

// IsRepositoryDirty checks if the repository has uncommitted changes.
func (s *Service) IsRepositoryDirty(_ context.Context, repoPath string) (bool, error) {
	s.logger.Info("checking if repository is dirty", zap.String("path", repoPath))

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		s.logger.Error("failed to open repository", zap.Error(err))
		return false, fmt.Errorf("%w: %w", ErrRepositoryNotFound, err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		s.logger.Error("failed to get worktree", zap.Error(err))
		return false, fmt.Errorf("%w: %w", ErrInvalidRepository, err)
	}

	status, err := worktree.Status()
	if err != nil {
		s.logger.Error("failed to get worktree status", zap.Error(err))
		return false, fmt.Errorf("%w: %w", ErrInvalidRepository, err)
	}

	isDirty := !status.IsClean()
	s.logger.Info("repository dirty check", zap.String("path", repoPath), zap.Bool("isDirty", isDirty))
	return isDirty, nil
}

// GetRepositoryStatus returns detailed status information about the repository.
func (s *Service) GetRepositoryStatus(_ context.Context, repoPath string) (*RepositoryStatus, error) {
	s.logger.Info("getting repository status", zap.String("path", repoPath))

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		s.logger.Error("failed to open repository", zap.Error(err))
		return nil, fmt.Errorf("%w: %w", ErrRepositoryNotFound, err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		s.logger.Error("failed to get worktree", zap.Error(err))
		return nil, fmt.Errorf("%w: %w", ErrInvalidRepository, err)
	}

	// Get current branch
	head, err := repo.Head()
	if err != nil {
		s.logger.Error("failed to get HEAD", zap.Error(err))
		return nil, fmt.Errorf("%w: %w", ErrInvalidRepository, err)
	}

	currentBranch := head.Name().Short()
	if head.Name().IsBranch() {
		currentBranch = head.Name().Short()
	}

	// Get remote URL
	remote, err := repo.Remote("origin")
	remoteURL := ""
	if err == nil {
		remoteURL = remote.Config().URLs[0]
	}

	// Get last commit
	commit, err := repo.CommitObject(head.Hash())
	lastCommit := ""
	lastCommitTime := time.Time{}
	if err == nil {
		lastCommit = commit.Hash.String()
		lastCommitTime = commit.Author.When
	}

	// Get worktree status
	status, err := worktree.Status()
	if err != nil {
		s.logger.Error("failed to get worktree status", zap.Error(err))
		return nil, fmt.Errorf("%w: %w", ErrInvalidRepository, err)
	}

	isDirty := !status.IsClean()
	var uncommittedChanges []string
	for path, fileStatus := range status {
		if fileStatus.Staging != git.Untracked || fileStatus.Worktree != git.Untracked {
			change := fmt.Sprintf("%s: staging=%c worktree=%c", path, fileStatus.Staging, fileStatus.Worktree)
			uncommittedChanges = append(uncommittedChanges, change)
		}
	}

	repoStatus := &RepositoryStatus{
		Path:               repoPath,
		IsDirty:            isDirty,
		CurrentBranch:      currentBranch,
		RemoteURL:          remoteURL,
		LastCommit:         lastCommit,
		LastCommitTime:     lastCommitTime,
		UncommittedChanges: uncommittedChanges,
	}

	s.logger.Info("repository status retrieved",
		zap.String("path", repoPath),
		zap.Bool("isDirty", isDirty),
		zap.String("branch", currentBranch),
		zap.Int("uncommittedChanges", len(uncommittedChanges)))

	return repoStatus, nil
}

// checkDiskSpace checks if there's enough disk space for operations.
func (s *Service) checkDiskSpace(_ context.Context, directory string) error {
	minSpace := s.config.Performance.MinDiskSpaceBytes
	if minSpace <= 0 {
		return nil // skip check if not configured
	}

	var stat syscall.Statfs_t
	err := syscall.Statfs(directory, &stat)
	if err != nil {
		s.logger.Warn("failed to check disk space", zap.Error(err))
		return nil // don't fail operation if we can't check
	}

	availableBytes := stat.Bavail * uint64(stat.Bsize)
	if uint64(minSpace) > availableBytes {
		return fmt.Errorf(
			"%w: insufficient disk space (available: %d bytes, required: %d bytes)",
			ErrDiskSpace,
			availableBytes,
			minSpace,
		)
	}

	return nil
}

// acquireOperationLock acquires a semaphore lock for concurrent operations.
func (s *Service) acquireOperationLock(ctx context.Context) error {
	if err := s.semaphore.Acquire(ctx, 1); err != nil {
		return fmt.Errorf("failed to acquire operation lock: %w", err)
	}

	return nil
}

// releaseOperationLock releases the semaphore lock.
func (s *Service) releaseOperationLock() {
	s.semaphore.Release(1)
}

// CloneWithProgress clones a repository with progress reporting.
func (s *Service) CloneWithProgress(
	ctx context.Context,
	req CloneRequest,
	progress ProgressCallback,
) (*Repository, error) {
	req.Progress = progress
	return s.Clone(ctx, req)
}

// PullWithProgress pulls changes with progress reporting.
func (s *Service) PullWithProgress(ctx context.Context, req PullRequest, progress ProgressCallback) error {
	req.Progress = progress
	return s.Pull(ctx, req)
}
