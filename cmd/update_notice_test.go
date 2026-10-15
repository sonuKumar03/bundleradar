package cmd

import (
	"bytes"
	"os"
	"testing"
)

func TestShouldCheckForUpdatesSkipsNonInteractiveOutput(t *testing.T) {
	t.Setenv("CI", "")
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("BUNDLERADAR_NO_UPDATE_CHECK", "")

	if shouldCheckForUpdates([]string{"scan"}, &bytes.Buffer{}) {
		t.Fatal("a buffer is not a terminal; the check must be skipped")
	}
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer devNull.Close()
	if shouldCheckForUpdates([]string{"scan"}, devNull) {
		t.Fatal("/dev/null must be treated as non-interactive")
	}
}

func TestShouldCheckForUpdatesSkipsQuietCommandsAndOptOut(t *testing.T) {
	t.Setenv("CI", "")
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("BUNDLERADAR_NO_UPDATE_CHECK", "1")
	if shouldCheckForUpdates([]string{"scan"}, os.Stderr) {
		t.Fatal("opt-out env var must disable the check")
	}
	t.Setenv("BUNDLERADAR_NO_UPDATE_CHECK", "")
	for _, c := range []string{"mcp", "update", "completion"} {
		if shouldCheckForUpdates([]string{c}, os.Stderr) {
			t.Fatalf("%s must never print update notices", c)
		}
	}
	t.Setenv("CI", "true")
	if shouldCheckForUpdates([]string{"scan"}, os.Stderr) {
		t.Fatal("CI must disable the check")
	}
}
