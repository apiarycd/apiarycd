package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
	"go.uber.org/zap/zaptest"
)

func TestService_CreateBranch(t *testing.T) {
	// Create temporary directory for test repo
	tempDir, err := os.MkdirTemp("", "git-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	repoPath := filepath.Join(tempDir, "repo")
	err = os.MkdirAll(repoPath, 0755)
	if err != nil {
		t.Fatal(err)
	}

	// Initialize a test git repository
	repo, err := git.PlainInit(repoPath, false)
	if err != nil {
		t.Fatal(err)
	}

	// Create initial commit
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}

	testFile := filepath.Join(repoPath, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	_, err = worktree.Add("test.txt")
	if err != nil {
		t.Fatal(err)
	}

	_, err = worktree.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test Author",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create service
	config := Config{}
	logger := zaptest.NewLogger(t)
	service := NewService(config, logger)

	// Test creating a new branch
	req := BranchCreateRequest{
		Path:     repoPath,
		Name:     "feature-branch",
		Checkout: true,
	}

	branchInfo, err := service.CreateBranch(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}

	if branchInfo.Name != "feature-branch" {
		t.Errorf("Expected branch name 'feature-branch', got '%s'", branchInfo.Name)
	}

	if branchInfo.Hash == "" {
		t.Error("Expected branch to have a hash")
	}

	// Verify branch exists
	branches, err := service.GetBranches(context.Background(), repoPath)
	if err != nil {
		t.Fatalf("GetBranches failed: %v", err)
	}

	found := false
	for _, b := range branches {
		if b.Name == "feature-branch" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Created branch not found in branch list")
	}
}

func TestService_DeleteBranch(t *testing.T) {
	// Create temporary directory for test repo
	tempDir, err := os.MkdirTemp("", "git-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	repoPath := filepath.Join(tempDir, "repo")
	err = os.MkdirAll(repoPath, 0755)
	if err != nil {
		t.Fatal(err)
	}

	// Initialize a test git repository
	repo, err := git.PlainInit(repoPath, false)
	if err != nil {
		t.Fatal(err)
	}

	// Create initial commit
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}

	testFile := filepath.Join(repoPath, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	_, err = worktree.Add("test.txt")
	if err != nil {
		t.Fatal(err)
	}

	_, err = worktree.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test Author",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create a branch to delete
	err = worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("test-branch"),
		Create: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Switch back to master
	err = worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("master"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create service
	config := Config{}
	logger := zaptest.NewLogger(t)
	service := NewService(config, logger)

	// Test deleting the branch
	deleteReq := BranchDeleteRequest{
		Path: repoPath,
		Name: "test-branch",
	}

	err = service.DeleteBranch(context.Background(), deleteReq)
	if err != nil {
		t.Fatalf("DeleteBranch failed: %v", err)
	}

	// Verify branch is deleted
	branches, err := service.GetBranches(context.Background(), repoPath)
	if err != nil {
		t.Fatalf("GetBranches failed: %v", err)
	}

	for _, b := range branches {
		if b.Name == "test-branch" {
			t.Error("Branch should have been deleted but still exists")
			break
		}
	}
}

func TestService_CreateTag(t *testing.T) {
	// Create temporary directory for test repo
	tempDir, err := os.MkdirTemp("", "git-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	repoPath := filepath.Join(tempDir, "repo")
	err = os.MkdirAll(repoPath, 0755)
	if err != nil {
		t.Fatal(err)
	}

	// Initialize a test git repository
	repo, err := git.PlainInit(repoPath, false)
	if err != nil {
		t.Fatal(err)
	}

	// Create initial commit
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}

	testFile := filepath.Join(repoPath, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	_, err = worktree.Add("test.txt")
	if err != nil {
		t.Fatal(err)
	}

	_, err = worktree.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test Author",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create service
	config := Config{}
	logger := zaptest.NewLogger(t)
	service := NewService(config, logger)

	// Test creating a lightweight tag
	req := TagCreateRequest{
		Path:      repoPath,
		Name:      "v1.0.0",
		Annotated: false,
	}

	tagInfo, err := service.CreateTag(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateTag failed: %v", err)
	}

	if tagInfo.Name != "v1.0.0" {
		t.Errorf("Expected tag name 'v1.0.0', got '%s'", tagInfo.Name)
	}

	if tagInfo.Hash == "" {
		t.Error("Expected tag to have a hash")
	}

	// Verify tag exists
	tags, err := service.GetTags(context.Background(), repoPath)
	if err != nil {
		t.Fatalf("GetTags failed: %v", err)
	}

	found := false
	for _, tag := range tags {
		if tag.Name == "v1.0.0" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Created tag not found in tag list")
	}
}

func TestService_GetBranchesFiltered(t *testing.T) {
	// Create temporary directory for test repo
	tempDir, err := os.MkdirTemp("", "git-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	repoPath := filepath.Join(tempDir, "repo")
	err = os.MkdirAll(repoPath, 0755)
	if err != nil {
		t.Fatal(err)
	}

	// Initialize a test git repository
	repo, err := git.PlainInit(repoPath, false)
	if err != nil {
		t.Fatal(err)
	}

	// Create initial commit
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}

	testFile := filepath.Join(repoPath, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	_, err = worktree.Add("test.txt")
	if err != nil {
		t.Fatal(err)
	}

	_, err = worktree.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test Author",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create multiple branches
	branches := []string{"feature-1", "feature-2", "bugfix-1", "master"}
	for _, branchName := range branches[:len(branches)-1] { // Skip master as it already exists
		err = worktree.Checkout(&git.CheckoutOptions{
			Branch: plumbing.NewBranchReferenceName(branchName),
			Create: true,
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// Switch back to master
	err = worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("master"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create service
	config := Config{}
	logger := zaptest.NewLogger(t)
	service := NewService(config, logger)

	// Test filtering branches with pattern
	req := BranchFilterRequest{
		Path: repoPath,
		Filter: ReferenceFilter{
			Pattern: "feature-*",
		},
		Sort: ReferenceSort{
			By:    "name",
			Order: "asc",
		},
	}

	filteredBranches, err := service.GetBranchesFiltered(context.Background(), req)
	if err != nil {
		t.Fatalf("GetBranchesFiltered failed: %v", err)
	}

	// Should find 2 feature branches
	if len(filteredBranches) != 2 {
		t.Errorf("Expected 2 filtered branches, got %d", len(filteredBranches))
	}

	// Check names
	expectedNames := []string{"feature-1", "feature-2"}
	for i, branch := range filteredBranches {
		if branch.Name != expectedNames[i] {
			t.Errorf("Expected branch name '%s', got '%s'", expectedNames[i], branch.Name)
		}
	}
}
