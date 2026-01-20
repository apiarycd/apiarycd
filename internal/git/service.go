package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/go-git/go-git/v6/plumbing/transport"
	"github.com/go-git/go-git/v6/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v6/plumbing/transport/ssh"
	"go.uber.org/zap"
)

type Service struct {
	config Config

	logger *zap.Logger
}

// NewService creates a new GitService.
func NewService(config Config, logger *zap.Logger) *Service {
	return &Service{
		config: config,

		logger: logger,
	}
}

// buildAuth converts an Authenticator to a go-git authentication object.
func (s *Service) buildAuth(auth Authenticator) (transport.AuthMethod, error) {
	if auth == nil {
		return nil, nil
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
func (s *Service) buildSSHAuth(auth *SSHAuth) (*gitssh.PublicKeys, error) {
	privateKeyPath := auth.PrivateKeyPath
	if privateKeyPath == "" {
		privateKeyPath = s.config.Auth.SSH.DefaultPrivateKey
	}

	if privateKeyPath == "" {
		return nil, fmt.Errorf("SSH private key path is required")
	}

	keys, err := gitssh.NewPublicKeysFromFile("git", privateKeyPath, auth.Passphrase)
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

	cloneOptions := &git.CloneOptions{
		URL:          req.URL,
		SingleBranch: true,
		Depth:        1,
	}

	if req.Branch != "" {
		cloneOptions.ReferenceName = plumbing.NewBranchReferenceName(req.Branch)
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

	_, err := git.PlainCloneContext(ctx, req.Directory, cloneOptions)
	if err != nil {
		s.logger.Error("failed to clone repository", zap.Error(err))
		return nil, fmt.Errorf("%w: %w", ErrCloneFailed, err)
	}

	s.logger.Info("repository cloned successfully",
		zap.String("url", req.URL),
		zap.String("directory", req.Directory))

	return &Repository{
		Path: req.Directory,
		URL:  req.URL,
	}, nil
}

// Pull pulls the latest changes for the specified repository.
func (s *Service) Pull(ctx context.Context, req PullRequest) error {
	s.logger.Info("pulling repository",
		zap.String("path", req.Path),
		zap.String("branch", req.Branch))

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

	pullOptions := &git.PullOptions{
		RemoteName:   "origin",
		SingleBranch: true,
		Depth:        1,
	}

	if req.Branch != "" {
		pullOptions.ReferenceName = plumbing.NewBranchReferenceName(req.Branch)
	}

	// Set up authentication
	if req.Auth != nil {
		auth, err := s.buildAuth(req.Auth)
		if err != nil {
			return fmt.Errorf("failed to build authentication: %w", err)
		}
		pullOptions.Auth = auth
	}

	err = worktree.PullContext(ctx, pullOptions)
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		s.logger.Error("failed to pull repository", zap.Error(err))
		return fmt.Errorf("%w: %w", ErrPullFailed, err)
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
