package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/sonuKumar03/bundleradar/internal/adapters/server"
	"github.com/sonuKumar03/bundleradar/internal/core"
	"github.com/sonuKumar03/bundleradar/pkg/bundleradar"
)

func TestServer_GetBundle_Success(t *testing.T) {
	fixturePath := "../../../testdata/nx-workspace/apps/portal/stats.json"
	if _, err := os.Stat(fixturePath); os.IsNotExist(err) {
		fixturePath = "../../../testdata/nx-workspace/dist/apps/portal/stats.json"
	}

	client := bundleradar.New()
	srv, err := server.New(server.Config{
		Host:      "127.0.0.1",
		Port:      0,
		StatsPath: fixturePath,
		Client:    client,
	})
	if err != nil {
		t.Fatalf("server.New failed: %v", err)
	}

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/bundle")
	if err != nil {
		t.Fatalf("GET /api/bundle failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	var data struct {
		Bundler     string `json:"bundler"`
		StatsPath   string `json:"statsPath"`
		Entrypoints map[string]struct {
			InitialBytes     int64    `json:"initialBytes"`
			InitialGzipBytes int64    `json:"initialGzipBytes"`
			AsyncBytes       int64    `json:"asyncBytes"`
			ChunkIds         []string `json:"chunkIds"`
		} `json:"entrypoints"`
		Chunks []struct {
			Name      string `json:"name"`
			Type      string `json:"type"`
			SizeBytes int64  `json:"sizeBytes"`
			GzipBytes int64  `json:"gzipBytes"`
		} `json:"chunks"`
		TopPackages []struct {
			Name        string   `json:"name"`
			SizeBytes   int64    `json:"sizeBytes"`
			GzipBytes   int64    `json:"gzipBytes"`
			Chunks      []string `json:"chunks"`
			IngressPath string   `json:"ingressPath"`
		} `json:"topPackages"`
		TotalBytes int64 `json:"totalBytes"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("json decode failed: %v", err)
	}

	if len(data.Chunks) == 0 {
		t.Errorf("expected non-empty chunks list")
	}
	if len(data.TopPackages) == 0 {
		t.Errorf("expected non-empty topPackages list")
	}

	// Verify top packages have sizes and some packages have ingress attribution
	hasIngress := false
	for _, pkg := range data.TopPackages {
		if pkg.IngressPath != "" {
			hasIngress = true
			break
		}
	}
	if !hasIngress {
		t.Errorf("expected at least one package with ingressPath attribution")
	}
}

func TestBundleToDTOUsesPerChunkModuleBytes(t *testing.T) {
	srv, err := server.New(server.Config{Host: "127.0.0.1", Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	b := core.NewBundle(core.Metadata{})
	b.AddChunk(core.Chunk{ID: "main.js", Name: "main.js", Type: core.LoadTypeInitial})
	b.AddChunk(core.Chunk{ID: "lazy.js", Name: "lazy.js", Type: core.LoadTypeAsync})
	b.AddModule(core.Module{
		ID: "node_modules/lodash/index.js", Package: "lodash", SizeBytes: 1000,
		ChunkIDs: []string{"main.js", "lazy.js"}, ChunkBytes: map[string]int64{"main.js": 100, "lazy.js": 900},
	})

	for _, pkg := range srv.BundleToDTO(b).TopPackages {
		if pkg.Name == "lodash" {
			if pkg.SizeBytes != 1000 || pkg.InitialBytes != 100 || pkg.AsyncBytes != 900 {
				t.Fatalf("package bytes size=%d initial=%d async=%d, want 1000/100/900", pkg.SizeBytes, pkg.InitialBytes, pkg.AsyncBytes)
			}
			return
		}
	}
	t.Fatal("lodash package missing from bundle DTO")
}

func TestServer_GetBundle_NotFound(t *testing.T) {
	srv, err := server.New(server.Config{
		Host: "127.0.0.1",
		Port: 0,
	})
	if err != nil {
		t.Fatalf("server.New failed: %v", err)
	}

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/bundle")
	if err != nil {
		t.Fatalf("GET /api/bundle failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found, got %d", resp.StatusCode)
	}
}

func TestServer_GetBundle_PreSetBundle(t *testing.T) {
	srv, err := server.New(server.Config{
		Host: "127.0.0.1",
		Port: 0,
	})
	if err != nil {
		t.Fatalf("server.New failed: %v", err)
	}

	bundle := core.NewBundle(core.Metadata{Bundler: "test"})
	bundle.AddChunk(core.Chunk{
		ID:        "main.js",
		Name:      "main.js",
		SizeBytes: 1000,
		Type:      core.LoadTypeInitial,
	})
	bundle.AddModule(core.Module{
		ID:           "node_modules/lodash/index.js",
		Package:      "lodash",
		SizeBytes:    500,
		ChunkIDs:     []string{"main.js"},
		IngressPaths: []string{"src/index.js", "lodash"},
	})
	srv.SetBundle(bundle)

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/bundle")
	if err != nil {
		t.Fatalf("GET /api/bundle failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	var data struct {
		Bundler     string `json:"bundler"`
		TopPackages []struct {
			Name        string `json:"name"`
			IngressPath string `json:"ingressPath"`
		} `json:"topPackages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("json decode failed: %v", err)
	}

	if data.Bundler != "test" {
		t.Errorf("expected bundler 'test', got %s", data.Bundler)
	}
	if len(data.TopPackages) != 1 || data.TopPackages[0].Name != "lodash" {
		t.Fatalf("expected top package lodash, got %+v", data.TopPackages)
	}
	if data.TopPackages[0].IngressPath != "src/index.js → lodash" {
		t.Errorf("expected ingress path 'src/index.js → lodash', got %s", data.TopPackages[0].IngressPath)
	}
}
