package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/sonuKumar03/bundleradar/cmd"
)

func TestDiffAcceptsScanJSONBaseline(t *testing.T) {
	stats := filepath.Join("..", "testdata", "minimal", "stats.json")
	var scanOut, errs bytes.Buffer
	if code := cmd.Execute([]string{"scan", stats, "--bundler", "esbuild", "-f", "json"}, &scanOut, &errs); code != cmd.ExitCodeSuccess {
		t.Fatalf("scan: code=%d error=%s", code, errs.String())
	}
	baseline := filepath.Join(t.TempDir(), "baseline.json")
	if err := os.WriteFile(baseline, scanOut.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	var diffOut bytes.Buffer
	errs.Reset()
	if code := cmd.Execute([]string{"diff", stats, "--against", baseline, "-f", "json"}, &diffOut, &errs); code != cmd.ExitCodeSuccess {
		t.Fatalf("diff: code=%d output=%s error=%s", code, diffOut.String(), errs.String())
	}
}
