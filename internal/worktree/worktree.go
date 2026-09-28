// Package worktree manages temporary git worktrees for isolated branch builds.
package worktree

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// IsGitRepo checks if the specified directory is inside a git repository.
func IsGitRepo(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}

// GetRepoRoot returns the top-level directory of the git repository.
func GetRepoRoot(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("determine git repository root: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ResolveRef resolves a git ref in repoRoot, checking if ref exists directly or
// falling back to origin/<ref> if the ref is only available remotely (common in CI PR checkouts).
func ResolveRef(dir string, ref string) (string, error) {
	if !IsGitRepo(dir) {
		return "", fmt.Errorf("directory %q is not a git repository", dir)
	}
	repoRoot, err := GetRepoRoot(dir)
	if err != nil {
		return "", err
	}

	cmd := exec.Command("git", "rev-parse", "--verify", ref)
	cmd.Dir = repoRoot
	if err := cmd.Run(); err == nil {
		return ref, nil
	}

	if !strings.HasPrefix(ref, "origin/") {
		originRef := "origin/" + ref
		cmdOrigin := exec.Command("git", "rev-parse", "--verify", originRef)
		cmdOrigin.Dir = repoRoot
		if err := cmdOrigin.Run(); err == nil {
			return originRef, nil
		}
	}

	return ref, nil
}

// GetCommitSHA resolves a git ref (branch, tag, commit) to a short commit SHA.
func GetCommitSHA(dir string, ref string) (string, error) {
	resolvedRef, _ := ResolveRef(dir, ref)
	if resolvedRef == "" {
		resolvedRef = ref
	}
	cmd := exec.Command("git", "rev-parse", "--short", resolvedRef)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("resolve git ref %q: %w", ref, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// Create creates a temporary git worktree checked out to the given ref.
// It returns the worktree directory path, a cleanup function, and any error encountered.
func Create(rootDir string, ref string) (string, func(), error) {
	if !IsGitRepo(rootDir) {
		return "", nil, fmt.Errorf("directory %q is not a git repository", rootDir)
	}

	repoRoot, err := GetRepoRoot(rootDir)
	if err != nil {
		return "", nil, err
	}

	tempDir, err := os.MkdirTemp("", "bundleradar-wt-*")
	if err != nil {
		return "", nil, fmt.Errorf("create temp worktree dir: %w", err)
	}

	resolvedRef, _ := ResolveRef(repoRoot, ref)
	if resolvedRef == "" {
		resolvedRef = ref
	}

	// Add git worktree
	cmd := exec.Command("git", "worktree", "add", "--detach", tempDir, resolvedRef)
	cmd.Dir = repoRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		_ = os.RemoveAll(tempDir)
		return "", nil, fmt.Errorf("git worktree add failed for ref %q: %s", ref, strings.TrimSpace(stderr.String()))
	}

	// Symlink node_modules from repoRoot if present to avoid npm install in worktree
	_ = SymlinkNodeModules(repoRoot, tempDir)

	cleanup := func() {
		rmCmd := exec.Command("git", "worktree", "remove", "--force", tempDir)
		rmCmd.Dir = repoRoot
		_ = rmCmd.Run()
		_ = os.RemoveAll(tempDir)
	}

	return tempDir, cleanup, nil
}

// SymlinkNodeModules links srcDir/node_modules into destDir/node_modules when
// the source exists and the destination does not. Callers use it to reuse a
// real checkout's installed dependencies inside a temporary worktree, for both
// the repo root and nested project directories (e.g. monorepo subfolders).
func SymlinkNodeModules(srcDir, destDir string) error {
	srcNodeModules := filepath.Join(srcDir, "node_modules")
	destNodeModules := filepath.Join(destDir, "node_modules")
	fi, err := os.Stat(srcNodeModules)
	if err != nil || !fi.IsDir() {
		return nil
	}
	if _, err := os.Stat(destNodeModules); !os.IsNotExist(err) {
		return nil
	}
	if err := os.Symlink(srcNodeModules, destNodeModules); err != nil {
		return fmt.Errorf("symlink node_modules %q -> %q: %w", srcNodeModules, destNodeModules, err)
	}
	return nil
}

// FindProjectDir locates the directory inside worktreeDir that owns the build,
// given the stats file path relative to the worktree root. It walks up from the
// stats file's directory to the worktree root and returns the nearest ancestor
// containing a package.json, falling back to worktreeDir itself. This supports
// monorepo layouts where the buildable project lives below the repo root.
func FindProjectDir(worktreeDir, statsRelPath string) string {
	startDir := filepath.Dir(filepath.Join(worktreeDir, statsRelPath))
	if !strings.HasPrefix(filepath.ToSlash(startDir)+"/", filepath.ToSlash(worktreeDir)+"/") {
		return worktreeDir
	}

	dir := startDir
	for {
		if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
			return dir
		}
		if filepath.ToSlash(dir) == filepath.ToSlash(worktreeDir) {
			return worktreeDir
		}
		parent := filepath.Dir(dir)
		if filepath.ToSlash(parent) == filepath.ToSlash(dir) {
			return worktreeDir
		}
		dir = parent
	}
}

// RunBuild executes the build command within the specified directory.
// The directory is typically the worktree project dir returned by FindProjectDir.
func RunBuild(workDir string, buildCmd string) error {
	if strings.TrimSpace(buildCmd) == "" {
		pkgJSON := filepath.Join(workDir, "package.json")
		if _, err := os.Stat(pkgJSON); err == nil {
			buildCmd = "npm run build"
		} else {
			return fmt.Errorf("no build command provided and package.json not found in %q", workDir)
		}
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd.exe", "/c", buildCmd)
	} else {
		cmd = exec.Command("sh", "-c", buildCmd)
	}

	cmd.Dir = workDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errOutput := strings.TrimSpace(stderr.String())
		if errOutput == "" {
			errOutput = strings.TrimSpace(stdout.String())
		}
		return fmt.Errorf("build command %q failed:\n%s", buildCmd, errOutput)
	}

	return nil
}