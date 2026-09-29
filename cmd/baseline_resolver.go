package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sonuKumar03/bundleradar/internal/core"
	"github.com/sonuKumar03/bundleradar/internal/worktree"
	"github.com/sonuKumar03/bundleradar/pkg/bundleradar"
)

// resolveBaselineBundle resolves and scans a baseline bundle from either a local file or a git ref.
func resolveBaselineBundle(
	ctx context.Context,
	client *bundleradar.Client,
	against string,
	currentStatsPath string,
	bundler string,
	buildCmd string,
	buildDir string,
	noBuild bool,
	out io.Writer,
	format string,
) (*bundleradar.Bundle, func(), error) {
	noopCleanup := func() {}

	// 1. Check if against is an existing file on disk
	if fi, err := os.Stat(against); err == nil && !fi.IsDir() {
		f, err := os.Open(against)
		if err != nil {
			return nil, noopCleanup, err
		}
		defer f.Close()
		header := make([]byte, 4096)
		n, _ := f.Read(header)
		if bytes.Contains(header[:n], []byte(`"metadata"`)) && bytes.Contains(header[:n], []byte(`"entrypoints"`)) {
			if _, err := f.Seek(0, 0); err != nil {
				return nil, noopCleanup, err
			}
			var base core.Bundle
			if err := json.NewDecoder(f).Decode(&base); err == nil && base.Metadata.Bundler != "" && base.Entrypoints != nil {
				return &base, noopCleanup, nil
			}
		}
		baseBundle, err := client.Scan(ctx, bundleradar.ScanOptions{
			StatsPath: against,
			Bundler:   bundler,
		})
		if err != nil {
			return nil, noopCleanup, fmt.Errorf("scan baseline bundle %q: %w", against, err)
		}
		return baseBundle, noopCleanup, nil
	}

	// 2. Fall back to git worktree resolution
	wd, err := os.Getwd()
	if err != nil {
		return nil, noopCleanup, fmt.Errorf("get working directory: %w", err)
	}
	if !worktree.IsGitRepo(wd) {
		return nil, noopCleanup, fmt.Errorf("baseline %q is neither a file nor was a git repository detected: %w", against, err)
	}

	repoRoot, err := worktree.GetRepoRoot(wd)
	if err != nil {
		return nil, noopCleanup, fmt.Errorf("get repo root: %w", err)
	}

	resolvedRef, err := worktree.ResolveRef(repoRoot, against)
	if err != nil {
		return nil, noopCleanup, fmt.Errorf("resolve git ref %q: %w", against, err)
	}

	wtDir, cleanup, err := worktree.Create(repoRoot, resolvedRef)
	if err != nil {
		return nil, noopCleanup, fmt.Errorf("create worktree for %q: %w", against, err)
	}

	// Map the current stats path to its location inside the worktree, relative
	// to the repo root. This keeps worktree comparisons correct when the CLI is
	// invoked from a nested project directory.
	statsRel := currentStatsPath
	if !filepath.IsAbs(currentStatsPath) {
		if rel, relErr := filepath.Rel(repoRoot, filepath.Join(wd, currentStatsPath)); relErr == nil && !strings.HasPrefix(rel, "..") {
			statsRel = rel
		}
	}
	targetStatsInWt := filepath.Join(wtDir, statsRel)

	// The build must run in the directory that owns package.json. Infer it by
	// walking up from the stats location (monorepo support), or honor an
	// explicit --build-dir override relative to the worktree root.
	var buildWorkDir string
	if buildDir != "" {
		buildWorkDir = filepath.Join(wtDir, buildDir)
		if _, err := os.Stat(buildWorkDir); err != nil {
			cleanup()
			return nil, noopCleanup, fmt.Errorf("--build-dir %q does not exist in ref %q: %w", buildDir, resolvedRef, err)
		}
	} else {
		buildWorkDir = worktree.FindProjectDir(wtDir, filepath.ToSlash(statsRel))
	}
	if rel, relErr := filepath.Rel(wtDir, buildWorkDir); relErr == nil {
		_ = worktree.SymlinkNodeModules(filepath.Join(repoRoot, rel), buildWorkDir)
	}

	if !noBuild {
		cmdToRun := buildCmd
		if cmdToRun == "" {
			cmdToRun = "npm run build"
		}
		if format != "json" && out != nil {
			fmt.Fprintf(out, "Building %q in temporary worktree (%s)...\n", resolvedRef, cmdToRun)
		}
		if err := worktree.RunBuild(buildWorkDir, cmdToRun); err != nil {
			cleanup()
			return nil, noopCleanup, fmt.Errorf("worktree build failed: %w", err)
		}
	}

	if _, err := os.Stat(targetStatsInWt); err != nil {
		cleanup()
		if noBuild {
			return nil, noopCleanup, fmt.Errorf("baseline stats %q not found in ref %q worktree: %w\nhint: the stats/dist output is likely gitignored; rerun without --no-build so the baseline is built inside the worktree", targetStatsInWt, resolvedRef, err)
		}
		return nil, noopCleanup, fmt.Errorf("baseline stats %q not found in ref %q worktree after build: %w\nhint: ensure the build command emits stats at this path, or pass --build-dir if the project lives in a subdirectory", targetStatsInWt, resolvedRef, err)
	}

	baseBundle, err := client.Scan(ctx, bundleradar.ScanOptions{
		StatsPath: targetStatsInWt,
		Bundler:   bundler,
	})
	if err != nil {
		cleanup()
		return nil, noopCleanup, fmt.Errorf("scan worktree baseline bundle (%s): %w", targetStatsInWt, err)
	}

	return baseBundle, cleanup, nil
}
