package cmd_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sonuKumar03/bundleradar/cmd"
)

func twoEntryStats(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stats.json")
	data := `{"inputs":{},"outputs":{"a.js":{"bytes":100,"entryPoint":"src/a.ts","imports":[],"inputs":{"src/a.ts":{"bytesInOutput":100}}},"b.js":{"bytes":1000,"entryPoint":"src/b.ts","imports":[],"inputs":{"node_modules/lodash/index.js":{"bytesInOutput":900}}}}}`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func gateStatsWithLazyBytes(t *testing.T, lazyBytes int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stats.json")
	data := fmt.Sprintf(`{"inputs":{},"outputs":{"main.js":{"bytes":100,"entryPoint":"src/main.ts","imports":[{"path":"lazy.js","kind":"dynamic-import"}],"inputs":{"src/main.ts":{"bytesInOutput":100}}},"lazy.js":{"bytes":%d,"imports":[],"inputs":{"node_modules/lodash/index.js":{"bytesInOutput":%d}}}}}`, lazyBytes, lazyBytes)
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestGateLoadsConfigAndFlagsOverride(t *testing.T) {
	stats := twoEntryStats(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".bundleradar.yml"), []byte("budgets:\n  total_max: 1B\n"), 0600); err != nil {
		t.Fatal(err)
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })

	var out, errs bytes.Buffer
	if code := cmd.Execute([]string{"gate", stats, "--bundler", "esbuild", "-f", "json"}, &out, &errs); code != cmd.ExitCodePolicyViolation {
		t.Fatalf("config budget: code=%d output=%s error=%s", code, out.String(), errs.String())
	}
	out.Reset()
	errs.Reset()
	if code := cmd.Execute([]string{"gate", stats, "--bundler", "esbuild", "--max-total", "2KB", "-f", "json"}, &out, &errs); code != cmd.ExitCodeSuccess {
		t.Fatalf("flag override: code=%d output=%s error=%s", code, out.String(), errs.String())
	}
}

func TestGateEntryScopesPackagesButKeepsTotal(t *testing.T) {
	stats := twoEntryStats(t)
	var out, errs bytes.Buffer
	code := cmd.Execute([]string{"gate", stats, "--bundler", "esbuild", "--entry", "src/a.ts", "--forbid", "lodash", "-f", "json"}, &out, &errs)
	if code != cmd.ExitCodeSuccess {
		t.Fatalf("other entry package should not fail: code=%d output=%s error=%s", code, out.String(), errs.String())
	}
	out.Reset()
	errs.Reset()
	code = cmd.Execute([]string{"gate", stats, "--bundler", "esbuild", "--entry", "src/a.ts", "--max-total", "200B", "-f", "json"}, &out, &errs)
	if code != cmd.ExitCodePolicyViolation {
		t.Fatalf("whole-build total must fail: code=%d output=%s error=%s", code, out.String(), errs.String())
	}
}

func TestGateDeltaNeedsBaseline(t *testing.T) {
	var out, errs bytes.Buffer
	code := cmd.Execute([]string{"gate", twoEntryStats(t), "--bundler", "esbuild", "--max-total-delta", "0B"}, &out, &errs)
	if code != cmd.ExitCodeUsage || !strings.Contains(errs.String(), "--against") {
		t.Fatalf("missing baseline: code=%d output=%s error=%s", code, out.String(), errs.String())
	}
}

func gateStatsWithCSSAndAssets(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stats.json")
	data := `{"inputs":{"src/styles.css":{"bytes":70000,"imports":[]},"images/logo.png":{"bytes":200000,"imports":[]}},"outputs":{"main.js":{"bytes":100,"entryPoint":"src/main.ts","imports":[],"inputs":{"src/main.ts":{"bytesInOutput":100}}},"styles.css":{"bytes":70000,"imports":[],"inputs":{"src/styles.css":{"bytesInOutput":70000}}},"images/logo.png":{"bytes":200000,"imports":[],"inputs":{"images/logo.png":{"bytesInOutput":200000}}}}}`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestGateCSSAndAssetBudgets(t *testing.T) {
	stats := gateStatsWithCSSAndAssets(t)
	var out, errs bytes.Buffer
	code := cmd.Execute([]string{"gate", stats, "--bundler", "esbuild", "--max-css", "50KB", "-f", "json"}, &out, &errs)
	if code != cmd.ExitCodePolicyViolation || !strings.Contains(out.String(), "MAX_CSS_SIZE") {
		t.Fatalf("css limit: code=%d output=%s error=%s", code, out.String(), errs.String())
	}
	out.Reset()
	errs.Reset()
	code = cmd.Execute([]string{"gate", stats, "--bundler", "esbuild", "--max-assets", "100KB", "-f", "json"}, &out, &errs)
	if code != cmd.ExitCodePolicyViolation || !strings.Contains(out.String(), "MAX_ASSETS_SIZE") {
		t.Fatalf("assets limit: code=%d output=%s error=%s", code, out.String(), errs.String())
	}
	out.Reset()
	errs.Reset()
	code = cmd.Execute([]string{"gate", stats, "--bundler", "esbuild", "--max-css", "1MB", "--max-assets", "1MB", "-f", "json"}, &out, &errs)
	if code != cmd.ExitCodeSuccess {
		t.Fatalf("asset budgets within limits should pass: code=%d output=%s error=%s", code, out.String(), errs.String())
	}
}

func TestGateLazyAndTotalDeltaBudgets(t *testing.T) {
	current, baseline := gateStatsWithLazyBytes(t, 200), gateStatsWithLazyBytes(t, 100)
	var out, errs bytes.Buffer
	code := cmd.Execute([]string{"gate", current, "--bundler", "esbuild", "--max-lazy", "199B", "-f", "json"}, &out, &errs)
	if code != cmd.ExitCodePolicyViolation || !strings.Contains(out.String(), "MAX_LAZY_SIZE") {
		t.Fatalf("lazy limit: code=%d output=%s error=%s", code, out.String(), errs.String())
	}
	out.Reset()
	errs.Reset()
	code = cmd.Execute([]string{"gate", current, "--bundler", "esbuild", "--against", baseline, "--max-total-delta", "99B", "-f", "json"}, &out, &errs)
	if code != cmd.ExitCodePolicyViolation || !strings.Contains(out.String(), "MAX_TOTAL_DELTA") {
		t.Fatalf("total delta limit: code=%d output=%s error=%s", code, out.String(), errs.String())
	}
}
