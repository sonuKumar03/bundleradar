package cmd_test

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sonuKumar03/bundleradar/cmd"
)

func run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := cmd.Execute(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestUnknownCommandIsUsageError(t *testing.T) {
	code, _, stderr := run(t, "nope")
	if code != cmd.ExitCodeUsage {
		t.Fatalf("unknown command: want exit %d, got %d (%s)", cmd.ExitCodeUsage, code, stderr)
	}
	if !strings.Contains(stderr, "unknown command") {
		t.Fatalf("unknown command: missing message: %s", stderr)
	}
}

func TestCompletionUnknownShellIsUsageError(t *testing.T) {
	code, _, stderr := run(t, "completion", "tcsh")
	if code != cmd.ExitCodeUsage {
		t.Fatalf("completion tcsh: want exit %d, got %d (%s)", cmd.ExitCodeUsage, code, stderr)
	}
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		// cobra writes completion scripts straight to os.Stdout, so only the
		// exit code is observable in-process.
		if code, _, stderr := run(t, "completion", shell); code != cmd.ExitCodeSuccess {
			t.Fatalf("completion %s: code=%d err=%s", shell, code, stderr)
		}
	}
}

func TestScanWhyUnknownPackageFails(t *testing.T) {
	stats := filepath.Join("..", "testdata", "minimal", "stats.json")
	code, out, stderr := run(t, "scan", stats, "--why", "not-a-real-pkg-xyz")
	if code != cmd.ExitCodeUsage {
		t.Fatalf("--why unknown: want exit %d, got %d", cmd.ExitCodeUsage, code)
	}
	if !strings.Contains(out, "not found") || !strings.Contains(stderr, "not found") {
		t.Fatalf("--why unknown: want not-found message, out=%q err=%q", out, stderr)
	}
	// json callers still get the structured result, with a failing exit code
	code, out, _ = run(t, "scan", stats, "--why", "not-a-real-pkg-xyz", "--format", "json")
	if code != cmd.ExitCodeUsage || !strings.Contains(out, `"found": false`) {
		t.Fatalf("--why unknown json: code=%d out=%s", code, out)
	}
	// a package that exists still succeeds
	if code, _, stderr := run(t, "scan", stats, "--why", "lodash"); code != cmd.ExitCodeSuccess {
		t.Fatalf("--why lodash: code=%d %s", code, stderr)
	}
}

func TestScanForcedWrongBundlerIsUsageError(t *testing.T) {
	stats := filepath.Join("..", "testdata", "minimal", "stats.json")
	code, _, stderr := run(t, "scan", stats, "--bundler", "vite")
	if code != cmd.ExitCodeUsage {
		t.Fatalf("--bundler vite on esbuild stats: want exit %d, got %d (%s)", cmd.ExitCodeUsage, code, stderr)
	}
	if !strings.Contains(stderr, "vite") {
		t.Fatalf("missing bundler name in message: %s", stderr)
	}
	// the matching override must keep working
	if code, _, stderr := run(t, "scan", stats, "--bundler", "esbuild"); code != cmd.ExitCodeSuccess {
		t.Fatalf("--bundler esbuild: code=%d %s", code, stderr)
	}
}

func TestWorkspaceRootMustExist(t *testing.T) {
	code, _, stderr := run(t, "workspace", "list", "--root", "/nonexistent-bundleradar-root")
	if code != cmd.ExitCodeUsage {
		t.Fatalf("missing root: want exit %d, got %d (%s)", cmd.ExitCodeUsage, code, stderr)
	}
	if !strings.Contains(stderr, "not found") || strings.Contains(stderr, "no supported workspace layout") {
		t.Fatalf("missing root: wrong message: %s", stderr)
	}
	// an existing directory without a workspace keeps the layout message
	code, _, stderr = run(t, "workspace", "list", "--root", t.TempDir())
	if code == cmd.ExitCodeSuccess || !strings.Contains(stderr, "no supported workspace layout") {
		t.Fatalf("empty root: code=%d err=%s", code, stderr)
	}
}
