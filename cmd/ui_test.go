package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestUICommand_FlagRegistration(t *testing.T) {
	cmd := newUICommand()
	if cmd.Use != "ui [stats.json]" {
		t.Errorf("unexpected Use: %s", cmd.Use)
	}

	flags := []string{"port", "host", "open", "watch"}
	for _, f := range flags {
		if cmd.Flags().Lookup(f) == nil {
			t.Errorf("expected flag --%s to be registered", f)
		}
	}
}

func TestScanCommand_UIFlagRegistration(t *testing.T) {
	cmd := newScanCommand()
	if cmd.Flags().Lookup("ui") == nil {
		t.Errorf("expected --ui flag on scan command")
	}
}

func TestAutoDiscoverStatsFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Initially no stats file
	found := autoDiscoverStatsFile(tmpDir)
	if found != "" {
		t.Errorf("expected empty string when no stats file, got %q", found)
	}

	// Create stats.json
	statsFile := filepath.Join(tmpDir, "stats.json")
	if err := os.WriteFile(statsFile, []byte("{}"), 0600); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	found = autoDiscoverStatsFile(tmpDir)
	if found != statsFile {
		t.Errorf("expected %q, got %q", statsFile, found)
	}
}

func TestRunUIServerWithContext(t *testing.T) {
	// Disable real browser launching in tests
	origLauncher := browserLauncher
	defer func() { browserLauncher = origLauncher }()
	browserLauncher = func(url string) error { return nil }

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel context to shutdown immediately

	statsPath := filepath.Join("..", "testdata", "minimal", "stats.json")
	err := runUIServerWithContext(ctx, "127.0.0.1", 0, statsPath, false, false, "")
	if err != nil {
		t.Fatalf("expected nil error on clean shutdown, got %v", err)
	}
}
