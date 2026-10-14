package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sonuKumar03/bundleradar/internal/adapters/server"
	"github.com/sonuKumar03/bundleradar/internal/core"
)

func TestServer_CheckpointsAndDiff(t *testing.T) {
	srv, err := server.New(server.Config{
		Host: "127.0.0.1",
		Port: 0,
	})
	if err != nil {
		t.Fatalf("server.New failed: %v", err)
	}

	// 1. Create Bundle 1 (Baseline)
	b1 := core.NewBundle(core.Metadata{Bundler: "test"})
	b1.AddChunk(core.Chunk{
		ID:        "main.js",
		Name:      "main.js",
		SizeBytes: 10000,
		Type:      core.LoadTypeInitial,
	})
	b1.AddEntrypoint("main", core.Entrypoint{
		InitialBytes: 10000,
		ChunkIDs:     []string{"main.js"},
	})
	b1.AddModule(core.Module{
		ID:        "node_modules/moment-timezone/index.js",
		Package:   "moment-timezone",
		SizeBytes: 4000,
		ChunkIDs:  []string{"main.js"},
	})
	b1.AddModule(core.Module{
		ID:        "node_modules/lodash/index.js",
		Package:   "lodash",
		SizeBytes: 2000,
		ChunkIDs:  []string{"main.js"},
	})

	cp1 := srv.RecordCheckpoint(b1, "Build #1 (Initial Baseline)")
	if cp1.ID == "" {
		t.Fatalf("expected non-empty checkpoint ID")
	}
	if !cp1.IsBaseline {
		t.Fatalf("first checkpoint should automatically be baseline")
	}

	// 2. Create Bundle 2 (Optimized: removed moment-timezone, reduced lodash)
	b2 := core.NewBundle(core.Metadata{Bundler: "test"})
	b2.AddChunk(core.Chunk{
		ID:        "main.js",
		Name:      "main.js",
		SizeBytes: 5000,
		Type:      core.LoadTypeInitial,
	})
	b2.AddEntrypoint("main", core.Entrypoint{
		InitialBytes: 5000,
		ChunkIDs:     []string{"main.js"},
	})
	b2.AddModule(core.Module{
		ID:        "node_modules/lodash/index.js",
		Package:   "lodash",
		SizeBytes: 1000,
		ChunkIDs:  []string{"main.js"},
	})

	cp2 := srv.RecordCheckpoint(b2, "Build #2 (Removed moment-timezone)")
	if cp2.ID == "" || cp2.ID == cp1.ID {
		t.Fatalf("expected unique non-empty checkpoint ID, got %s", cp2.ID)
	}

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 3. Test GET /api/history
	resp, err := http.Get(ts.URL + "/api/history")
	if err != nil {
		t.Fatalf("GET /api/history failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	var hist struct {
		BaselineID  string `json:"baselineId"`
		Checkpoints []struct {
			ID           string `json:"id"`
			InitialBytes int64  `json:"initialBytes"`
			IsBaseline   bool   `json:"isBaseline"`
		} `json:"checkpoints"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&hist); err != nil {
		t.Fatalf("decode history failed: %v", err)
	}
	if hist.BaselineID != cp1.ID {
		t.Errorf("expected baselineId %q, got %s", cp1.ID, hist.BaselineID)
	}
	if len(hist.Checkpoints) != 2 {
		t.Fatalf("expected 2 checkpoints, got %d", len(hist.Checkpoints))
	}

	// 4. Test GET /api/diff
	diffResp, err := http.Get(ts.URL + "/api/diff?base=" + cp1.ID + "&target=" + cp2.ID)
	if err != nil {
		t.Fatalf("GET /api/diff failed: %v", err)
	}
	defer diffResp.Body.Close()

	if diffResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", diffResp.StatusCode)
	}

	var diff struct {
		BaseID              string  `json:"baseId"`
		TargetID            string  `json:"targetId"`
		InitialDeltaBytes   int64   `json:"initialDeltaBytes"`
		InitialDeltaPercent float64 `json:"initialDeltaPercent"`
		PackageDiffs        []struct {
			Name              string `json:"name"`
			DeltaInitialBytes int64  `json:"deltaInitialBytes"`
			Status            string `json:"status"`
		} `json:"packageDiffs"`
	}
	if err := json.NewDecoder(diffResp.Body).Decode(&diff); err != nil {
		t.Fatalf("decode diff failed: %v", err)
	}

	if diff.InitialDeltaBytes != -5000 {
		t.Errorf("expected initial delta -5000, got %d", diff.InitialDeltaBytes)
	}
	if diff.InitialDeltaPercent != -50.0 {
		t.Errorf("expected initial delta percent -50.0, got %f", diff.InitialDeltaPercent)
	}

	// Verify moment-timezone is ELIMINATED
	foundMoment := false
	for _, p := range diff.PackageDiffs {
		if p.Name == "moment-timezone" {
			foundMoment = true
			if p.Status != "ELIMINATED" {
				t.Errorf("expected moment-timezone status ELIMINATED, got %s", p.Status)
			}
			if p.DeltaInitialBytes != -4000 {
				t.Errorf("expected moment-timezone delta -4000, got %d", p.DeltaInitialBytes)
			}
		}
	}
	if !foundMoment {
		t.Errorf("expected moment-timezone in package diffs")
	}

	// 5. Test POST /api/baseline
	baseReq, err := http.Post(ts.URL+"/api/baseline?id="+cp2.ID, "application/json", nil)
	if err != nil {
		t.Fatalf("POST /api/baseline failed: %v", err)
	}
	defer baseReq.Body.Close()
	if baseReq.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", baseReq.StatusCode)
	}

	if srv.BaselineID() != cp2.ID {
		t.Errorf("expected baselineID %q, got %s", cp2.ID, srv.BaselineID())
	}
}

func TestServer_EventsStream(t *testing.T) {
	srv, err := server.New(server.Config{
		Host: "127.0.0.1",
		Port: 0,
	})
	if err != nil {
		t.Fatalf("server.New failed: %v", err)
	}

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Connect SSE client
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/events", nil)
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("connect SSE failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Errorf("expected Content-Type text/event-stream, got %s", resp.Header.Get("Content-Type"))
	}
}
