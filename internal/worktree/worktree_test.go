package worktree_test

import (
	"os"
	"testing"

	"github.com/sonuKumar03/bundleradar/internal/worktree"
)

func TestIsGitRepo(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	if !worktree.IsGitRepo(wd) {
		t.Errorf("expected %q to be recognized as git repo", wd)
	}

	tmpDir := t.TempDir()
	if worktree.IsGitRepo(tmpDir) {
		t.Errorf("expected %q not to be git repo", tmpDir)
	}
}

func TestGetCommitSHA(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	sha, err := worktree.GetCommitSHA(wd, "HEAD")
	if err != nil {
		t.Fatalf("unexpected error getting HEAD sha: %v", err)
	}
	if len(sha) < 4 {
		t.Errorf("expected valid short SHA, got %q", sha)
	}
}

func TestCreateAndCleanup(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	wtDir, cleanup, err := worktree.Create(wd, "HEAD")
	if err != nil {
		t.Fatalf("failed to create worktree for HEAD: %v", err)
	}
	defer cleanup()

	if _, err := os.Stat(wtDir); os.IsNotExist(err) {
		t.Fatalf("worktree directory %q does not exist", wtDir)
	}

	// Test RunBuild with a harmless command inside the worktree
	if err := worktree.RunBuild(wtDir, "echo 'worktree build test'"); err != nil {
		t.Errorf("RunBuild failed: %v", err)
	}

	// Explicitly invoke cleanup
	cleanup()
	if _, err := os.Stat(wtDir); !os.IsNotExist(err) {
		t.Errorf("worktree directory %q still exists after cleanup", wtDir)
	}
}

func TestRunBuild_Failure(t *testing.T) {
	tmp := t.TempDir()
	err := worktree.RunBuild(tmp, "non_existent_command_12345")
	if err == nil {
		t.Error("expected error running nonexistent command")
	}
}

func TestResolveRef(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot, err := worktree.GetRepoRoot(wd)
	if err != nil {
		t.Fatalf("get repo root: %v", err)
	}

	// HEAD should always resolve
	ref, err := worktree.ResolveRef(repoRoot, "HEAD")
	if err != nil {
		t.Fatalf("resolve HEAD: %v", err)
	}
	if ref != "HEAD" {
		t.Errorf("expected HEAD, got %q", ref)
	}

	// Non-existent ref should return an error
	_, err = worktree.ResolveRef(repoRoot, "non-existent-ref-xyz")
	if err == nil {
		t.Fatal("expected error for non-existent ref, got nil")
	}
}

