package repositories_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/apiarycd/apiarycd/internal/repositories"
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
	runGit(t, "", "init", "--initial-branch=master", sourceDir)
	runGit(t, sourceDir, "config", "user.email", "test@example.com")
	runGit(t, sourceDir, "config", "user.name", "Test")

	filePath := filepath.Join(sourceDir, "README.md")
	if wrErr := os.WriteFile(filePath, []byte("hello\n"), 0600); wrErr != nil {
		t.Fatalf("failed to write file: %v", wrErr)
	}

	runGit(t, sourceDir, "add", "README.md")
	runGit(t, sourceDir, "commit", "-m", "initial commit")

	remoteDir := filepath.Join(baseDir, "remote.git")
	runGit(t, "", "init", "--bare", remoteDir)
	runGit(t, sourceDir, "remote", "add", "origin", remoteDir)
	runGit(t, sourceDir, "push", "origin", "HEAD")

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
	runGit(t, "", "init", "--bare", remoteDir)

	seedDir := filepath.Join(t.TempDir(), "seed")
	runGit(t, "", "init", "--initial-branch=master", seedDir)
	runGit(t, seedDir, "config", "user.email", "test@example.com")
	runGit(t, seedDir, "config", "user.name", "Test")

	writeAndCommit(t, seedDir, "README.md", "base", "initial commit")

	runGit(t, seedDir, "checkout", "-b", "feature")
	writeAndCommit(t, seedDir, "feature.txt", "feature", "feature commit")

	runGit(t, seedDir, "checkout", "master")
	runGit(t, seedDir, "remote", "add", "origin", remoteDir)
	runGit(t, seedDir, "push", "origin", "master", "feature")

	return remoteDir
}

func writeAndCommit(t *testing.T, repoDir, fileName, contents, message string) {
	t.Helper()

	filePath := filepath.Join(repoDir, fileName)
	if err := os.WriteFile(filePath, []byte(contents), 0600); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", fileName, err)
	}

	runGit(t, repoDir, "add", fileName)
	runGit(t, repoDir, "commit", "-m", message)
}

func currentHeadBranch(t *testing.T, repoDir string) string {
	t.Helper()

	out := runGit(t, repoDir, "branch", "--show-current")
	return strings.TrimSpace(out)
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(out))
	}

	return strings.TrimSpace(string(out))
}

func TestServiceGetDetails(t *testing.T) {
	ctx := context.Background()
	remoteURL := createBareRemote(t)
	svc := newTestService(t)

	repoID := uuid.New()
	req := repositories.CloneRequest{ID: repoID, URL: remoteURL}

	if err := svc.Clone(ctx, req); err != nil {
		t.Fatalf("clone failed: %v", err)
	}

	details, err := svc.GetDetails(ctx, repoID)
	if err != nil {
		t.Fatalf("GetDetails failed: %v", err)
	}

	if details.Path == "" || details.Commit == "" || details.CommitMessage == "" {
		t.Fatalf("unexpected details: %+v", details)
	}

	if !strings.Contains(details.CommitMessage, "initial commit") {
		t.Fatalf("expected commit message to contain 'initial commit', got %q", details.CommitMessage)
	}
}

func TestServiceBuildPath(t *testing.T) {
	svc := newTestService(t)
	id := uuid.MustParse("4f4f1a8f-0f18-4ad0-b90d-9474f5f5f85b")
	path := svc.BuildPath(id)

	if !strings.HasSuffix(path, id.String()) {
		t.Fatalf("BuildPath() = %q, expected suffix %q", path, id.String())
	}
}

func TestServiceDelete(t *testing.T) {
	svc := newTestService(t)
	id := uuid.New()
	dir := svc.BuildPath(id)

	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := svc.Delete(context.Background(), id); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected directory to be deleted, stat err = %v", err)
	}
}
