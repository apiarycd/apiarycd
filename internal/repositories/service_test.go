package repositories_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/apiarycd/apiarycd/internal/repositories"
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func TestServiceCloneOrPull_ExistingDirMatchingURL(t *testing.T) {
	ctx := context.Background()
	remoteURL := createBareRemote(t)
	svc := newTestService(t)

	repoID := uuid.New()
	req := repositories.CloneRequest{ID: repoID, URL: remoteURL}

	if err := svc.Clone(ctx, req); err != nil {
		t.Fatalf("clone failed: %v", err)
	}

	if err := svc.CloneOrPull(ctx, req); err != nil {
		t.Fatalf("clone or pull failed for matching URL: %v", err)
	}
}

func TestServiceCloneOrPull_ExistingDirMismatchedURL(t *testing.T) {
	ctx := context.Background()
	remoteURL := createBareRemote(t)
	mismatchURL := createBareRemote(t)
	svc := newTestService(t)

	repoID := uuid.New()
	if err := svc.Clone(ctx, repositories.CloneRequest{ID: repoID, URL: remoteURL}); err != nil {
		t.Fatalf("clone failed: %v", err)
	}

	err := svc.CloneOrPull(ctx, repositories.CloneRequest{ID: repoID, URL: mismatchURL})
	if !errors.Is(err, repositories.ErrValidationFailed) {
		t.Fatalf("expected ErrValidationFailed, got: %v", err)
	}
}

func TestServiceCloneOrPull_ExistingDirMissingURL(t *testing.T) {
	ctx := context.Background()
	remoteURL := createBareRemote(t)
	svc := newTestService(t)

	repoID := uuid.New()
	if err := svc.Clone(ctx, repositories.CloneRequest{ID: repoID, URL: remoteURL}); err != nil {
		t.Fatalf("clone failed: %v", err)
	}

	err := svc.CloneOrPull(ctx, repositories.CloneRequest{ID: repoID})
	if !errors.Is(err, repositories.ErrValidationFailed) {
		t.Fatalf("expected ErrValidationFailed, got: %v", err)
	}
}

func newTestService(t *testing.T) *repositories.Service {
	t.Helper()

	svc, err := repositories.NewService(repositories.Config{
		StorageDir: t.TempDir(),
		Timeout:    5 * time.Second,
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	return svc
}

func createBareRemote(t *testing.T) string {
	t.Helper()

	baseDir := t.TempDir()
	sourceDir := filepath.Join(baseDir, "source")
	sourceRepo, err := git.PlainInit(sourceDir, false)
	if err != nil {
		t.Fatalf("failed to init source repo: %v", err)
	}

	filePath := filepath.Join(sourceDir, "README.md")
	if wrErr := os.WriteFile(filePath, []byte("hello\n"), 0600); wrErr != nil {
		t.Fatalf("failed to write file: %v", wrErr)
	}

	worktree, err := sourceRepo.Worktree()
	if err != nil {
		t.Fatalf("failed to get source worktree: %v", err)
	}

	if _, addErr := worktree.Add("README.md"); addErr != nil {
		t.Fatalf("failed to add file: %v", addErr)
	}

	if _, comErr := worktree.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@example.com",
			When:  time.Now(),
		},
	}); comErr != nil {
		t.Fatalf("failed to commit: %v", comErr)
	}

	remoteDir := filepath.Join(baseDir, "remote.git")
	if _, initErr := git.PlainInit(remoteDir, true); initErr != nil {
		t.Fatalf("failed to init bare remote: %v", initErr)
	}

	if _, remErr := sourceRepo.CreateRemote(
		&config.RemoteConfig{Name: "origin", URLs: []string{remoteDir}},
	); remErr != nil {
		t.Fatalf("failed to create remote: %v", remErr)
	}

	if pushErr := sourceRepo.Push(&git.PushOptions{RemoteName: "origin"}); pushErr != nil {
		t.Fatalf("failed to push to remote: %v", pushErr)
	}

	return remoteDir
}

func TestServicePull_SameBranch(t *testing.T) {
	storageDir := t.TempDir()
	remotePath := setupRemoteRepository(t)

	svc, err := repositories.NewService(
		repositories.Config{StorageDir: storageDir, Timeout: 5 * time.Second},
		zap.NewNop(),
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	id := uuid.New()
	if err = svc.Clone(
		context.Background(),
		repositories.CloneRequest{ID: id, URL: remotePath, Branch: "master"},
	); err != nil {
		t.Fatalf("Clone() error = %v", err)
	}

	if err = svc.Pull(
		context.Background(),
		repositories.PullRequest{ID: id, URL: remotePath, Branch: "master"},
	); err != nil {
		t.Fatalf("Pull() error = %v", err)
	}

	head := currentHeadBranch(t, filepath.Join(storageDir, id.String()))
	if head != "master" {
		t.Fatalf("expected HEAD to remain on master, got %q", head)
	}
}

func TestServicePull_SwitchBranch(t *testing.T) {
	storageDir := t.TempDir()
	remotePath := setupRemoteRepository(t)

	svc, err := repositories.NewService(
		repositories.Config{StorageDir: storageDir, Timeout: 5 * time.Second},
		zap.NewNop(),
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	id := uuid.New()
	if err = svc.Clone(
		context.Background(),
		repositories.CloneRequest{ID: id, URL: remotePath, Branch: "master"},
	); err != nil {
		t.Fatalf("Clone() error = %v", err)
	}

	if err = svc.Pull(
		context.Background(),
		repositories.PullRequest{ID: id, URL: remotePath, Branch: "feature"},
	); err != nil {
		t.Fatalf("Pull() error = %v", err)
	}

	head := currentHeadBranch(t, filepath.Join(storageDir, id.String()))
	if head != "feature" {
		t.Fatalf("expected HEAD to switch to feature, got %q", head)
	}
}

func TestServicePull_NonExistentBranch(t *testing.T) {
	storageDir := t.TempDir()
	remotePath := setupRemoteRepository(t)

	svc, err := repositories.NewService(
		repositories.Config{StorageDir: storageDir, Timeout: 5 * time.Second},
		zap.NewNop(),
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	id := uuid.New()
	if err = svc.Clone(
		context.Background(),
		repositories.CloneRequest{ID: id, URL: remotePath, Branch: "master"},
	); err != nil {
		t.Fatalf("Clone() error = %v", err)
	}

	err = svc.Pull(context.Background(), repositories.PullRequest{ID: id, URL: remotePath, Branch: "does-not-exist"})
	if err == nil {
		t.Fatalf("expected Pull() to fail for non-existent branch")
	}

	if !errors.Is(err, repositories.ErrBranchNotFound) {
		t.Fatalf("expected ErrBranchNotFound, got %T (%v)", err, err)
	}

	if !strings.Contains(err.Error(), "does-not-exist") {
		t.Fatalf("expected missing branch name in error, got %q", err.Error())
	}
}

func setupRemoteRepository(t *testing.T) string {
	t.Helper()

	remoteDir := filepath.Join(t.TempDir(), "remote.git")
	_, err := git.PlainInit(remoteDir, true)
	if err != nil {
		t.Fatalf("PlainInit(remote, bare) error = %v", err)
	}

	seedDir := filepath.Join(t.TempDir(), "seed")
	repo, err := git.PlainInit(seedDir, false)
	if err != nil {
		t.Fatalf("PlainInit(seed) error = %v", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree() error = %v", err)
	}

	writeAndCommit(t, wt, seedDir, "README.md", "base", "initial commit")

	if err = wt.Checkout(
		&git.CheckoutOptions{Branch: plumbing.NewBranchReferenceName("feature"), Create: true},
	); err != nil {
		t.Fatalf("Checkout(feature) error = %v", err)
	}
	writeAndCommit(t, wt, seedDir, "feature.txt", "feature", "feature commit")

	if err = wt.Checkout(&git.CheckoutOptions{Branch: plumbing.NewBranchReferenceName("master")}); err != nil {
		t.Fatalf("Checkout(master) error = %v", err)
	}

	_, err = repo.CreateRemote(&config.RemoteConfig{Name: "origin", URLs: []string{remoteDir}})
	if err != nil {
		t.Fatalf("CreateRemote() error = %v", err)
	}

	err = repo.Push(&git.PushOptions{RemoteName: "origin", RefSpecs: []config.RefSpec{
		"refs/heads/master:refs/heads/master",
		"refs/heads/feature:refs/heads/feature",
	}})
	if err != nil {
		t.Fatalf("Push() error = %v", err)
	}

	return remoteDir
}

func writeAndCommit(t *testing.T, wt *git.Worktree, repoDir, fileName, contents, message string) {
	t.Helper()

	filePath := filepath.Join(repoDir, fileName)
	if err := os.WriteFile(filePath, []byte(contents), 0600); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", fileName, err)
	}

	if _, err := wt.Add(fileName); err != nil {
		t.Fatalf("Add(%s) error = %v", fileName, err)
	}

	if _, err := wt.Commit(message, &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@example.com", When: time.Now()},
	}); err != nil {
		t.Fatalf("Commit(%s) error = %v", message, err)
	}
}

func currentHeadBranch(t *testing.T, repoDir string) string {
	t.Helper()

	repo, err := git.PlainOpen(repoDir)
	if err != nil {
		t.Fatalf("PlainOpen() error = %v", err)
	}

	head, err := repo.Head()
	if err != nil {
		t.Fatalf("Head() error = %v", err)
	}

	return head.Name().Short()
}
