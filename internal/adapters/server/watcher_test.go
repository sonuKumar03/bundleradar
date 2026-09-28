package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sonuKumar03/bundleradar/internal/adapters/server"
	"github.com/sonuKumar03/bundleradar/pkg/bundleradar"
)

func TestServer_WatcherAndLiveEvent(t *testing.T) {
	// Create temporary directory with sample stats.json
	tmpDir := t.TempDir()
	statsFile := filepath.Join(tmpDir, "stats.json")

	initialStats := `{
		"inputs": {
			"src/main.ts": { "bytes": 5000, "imports": [] }
		},
		"outputs": {
			"dist/main.js": {
				"entryPoint": "src/main.ts",
				"bytes": 5000,
				"inputs": {
					"src/main.ts": { "bytesInOutput": 5000 }
				}
			}
		}
	}`
	if err := os.WriteFile(statsFile, []byte(initialStats), 0644); err != nil {
		t.Fatalf("failed to write initial stats: %v", err)
	}

	client := bundleradar.New()
	srv, err := server.New(server.Config{
		Host:      "127.0.0.1",
		Port:      0,
		StatsPath: statsFile,
		Client:    client,
		Watch:     true,
	})
	if err != nil {
		t.Fatalf("server.New failed: %v", err)
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start failed: %v", err)
	}
	defer func() { _ = srv.Close() }()

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Initial fetch to load bundle
	resp, err := http.Get(ts.URL + "/api/bundle")
	if err != nil {
		t.Fatalf("GET /api/bundle failed: %v", err)
	}
	resp.Body.Close()

	if srv.CheckpointCount() != 1 {
		t.Fatalf("expected 1 checkpoint initially, got %d", srv.CheckpointCount())
	}

	// Update stats file with a change
	time.Sleep(100 * time.Millisecond) // ensure mtime diff
	updatedStats := `{
		"inputs": {
			"src/main.ts": { "bytes": 8000, "imports": [] }
		},
		"outputs": {
			"dist/main.js": {
				"entryPoint": "src/main.ts",
				"bytes": 8000,
				"inputs": {
					"src/main.ts": { "bytesInOutput": 8000 }
				}
			}
		}
	}`
	if err := os.WriteFile(statsFile, []byte(updatedStats), 0644); err != nil {
		t.Fatalf("failed to write updated stats: %v", err)
	}

	// Wait for watcher to detect change (ticker is 400ms + 150ms debounce)
	deadline := time.Now().Add(2500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if srv.CheckpointCount() >= 2 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if srv.CheckpointCount() < 2 {
		t.Fatalf("expected at least 2 checkpoints after file modification, got %d", srv.CheckpointCount())
	}

	cp2 := srv.GetCheckpoint("build-2")
	if cp2 == nil {
		t.Fatalf("expected build-2 checkpoint")
	}
	if cp2.InitialBytes != 8000 {
		t.Errorf("expected build-2 initialBytes 8000, got %d", cp2.InitialBytes)
	}
}
