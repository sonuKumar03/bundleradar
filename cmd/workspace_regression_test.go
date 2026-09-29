package cmd_test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sonuKumar03/bundleradar/cmd"
)

func TestWorkspaceScanReportsFailedTargetsAndReturnsError(t *testing.T) {
	goodStats, err := filepath.Abs(filepath.Join("..", "testdata", "minimal", "stats.json"))
	if err != nil {
		t.Fatal(err)
	}
	missingStats := filepath.Join(t.TempDir(), "missing-stats.json")

	for _, format := range []string{"json", "markdown", "terminal"} {
		t.Run(format, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := cmd.Execute([]string{
				"workspace", "scan",
				"--app", "good=" + goodStats,
				"--app", "missing=" + missingStats,
				"-f", format,
			}, &stdout, &stderr)

			if format == "json" {
				var result struct {
					Targets []struct {
						Name  string `json:"name"`
						Error string `json:"error"`
					} `json:"targets"`
				}
				if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
					t.Fatalf("decode workspace result: %v; output=%s", err, stdout.String())
				}
				if len(result.Targets) != 2 || result.Targets[1].Name != "missing" || result.Targets[1].Error == "" {
					t.Fatalf("failed target missing from JSON: %+v", result.Targets)
				}
			} else if !strings.Contains(stdout.String(), "missing") || !strings.Contains(stdout.String(), "Error:") {
				t.Fatalf("failed target missing from output: %s", stdout.String())
			}

			if code != cmd.ExitCodeExecution {
				t.Fatalf("incomplete scan exit code=%d, want %d; stderr=%s", code, cmd.ExitCodeExecution, stderr.String())
			}
		})
	}
}
