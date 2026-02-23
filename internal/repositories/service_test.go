package repositories_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/apiarycd/apiarycd/internal/repositories"
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/config"
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
