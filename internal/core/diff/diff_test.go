package diff_test

import (
	"testing"

	"github.com/sonuKumar03/bundleradar/internal/core"
	"github.com/sonuKumar03/bundleradar/internal/core/diff"
)

func TestDiff_EntrypointsAndPackages(t *testing.T) {
	base := core.NewBundle(core.Metadata{Bundler: "test"})
	base.AddEntrypoint("main", core.Entrypoint{
		Name:             "main",
		InitialBytes:     100000,
		InitialGzipBytes: 30000,
		AsyncBytes:       50000,
		ChunkIDs:         []string{"main.js"},
	})
	base.AddChunk(core.Chunk{
		ID:        "main.js",
		Name:      "main.js",
		SizeBytes: 100000,
		GzipBytes: 30000,
		Type:      core.LoadTypeInitial,
	})
	base.AddModule(core.Module{
		ID:        "node_modules/lodash/map.js",
		Package:   "lodash",
		SizeBytes: 20000,
		GzipBytes: 6000,
	})

	current := core.NewBundle(core.Metadata{Bundler: "test"})
	current.AddEntrypoint("main", core.Entrypoint{
		Name:             "main",
		InitialBytes:     130000, // +30,000 bytes
		InitialGzipBytes: 39000,  // +9,000 bytes
		AsyncBytes:       50000,
		ChunkIDs:         []string{"main.js", "analytics.js"},
	})
	current.AddChunk(core.Chunk{
		ID:        "main.js",
		Name:      "main.js",
		SizeBytes: 110000,
		GzipBytes: 33000,
		Type:      core.LoadTypeInitial,
	})
	current.AddChunk(core.Chunk{
		ID:        "analytics.js",
		Name:      "analytics.js",
		SizeBytes: 20000,
		GzipBytes: 6000,
		Type:      core.LoadTypeInitial,
	})
	current.AddModule(core.Module{
		ID:        "node_modules/lodash/map.js",
		Package:   "lodash",
		SizeBytes: 20000,
		GzipBytes: 6000,
	})
	current.AddModule(core.Module{
		ID:        "node_modules/moment/moment.js",
		Package:   "moment",
		SizeBytes: 15000, // +15,000 bytes (new package)
		GzipBytes: 4500,
	})

	d := diff.Calculate(base, current, diff.Options{
		DriftThreshold: 1000, // 1KB
	})

	if ep, ok := d.Entrypoints["main"]; !ok {
		t.Fatalf("expected main entrypoint in diff")
	} else {
		if ep.InitialDelta != 30000 {
			t.Fatalf("expected initial delta 30000, got %d", ep.InitialDelta)
		}
		if ep.InitialGzipDelta != 9000 {
			t.Fatalf("expected gzip delta 9000, got %d", ep.InitialGzipDelta)
		}
	}

	if len(d.AddedChunks) != 1 || d.AddedChunks[0].ID != "analytics.js" {
		t.Fatalf("expected analytics.js in added chunks: %+v", d.AddedChunks)
	}

	if d.Summary.InitialDeltaBytes != 30000 {
		t.Fatalf("expected summary initial delta 30000, got %d", d.Summary.InitialDeltaBytes)
	}
	if d.Summary.BaseInitialBytes != 100000 || d.Summary.HeadInitialBytes != 130000 {
		t.Fatalf("unexpected summary base/head: %+v", d.Summary)
	}

	foundMoment := false
	for _, p := range d.Packages {
		if p.Name == "moment" {
			foundMoment = true
			if p.DeltaBytes != 15000 {
				t.Fatalf("expected moment delta 15000, got %d", p.DeltaBytes)
			}
		}
	}
	if !foundMoment {
		t.Fatalf("expected moment in package deltas: %+v", d.Packages)
	}

	foundLodashUnchanged := false
	for _, p := range d.UnchangedPackages {
		if p.Name == "lodash" {
			foundLodashUnchanged = true
			if p.CurrBytes != 20000 {
				t.Fatalf("expected lodash size 20000, got %d", p.CurrBytes)
			}
		}
	}
	if !foundLodashUnchanged {
		t.Fatalf("expected lodash in unchanged packages: %+v", d.UnchangedPackages)
	}

	if len(d.Attributions) == 0 || d.Attributions[0].SourceFile != "moment" {
		t.Fatalf("expected moment attribution: %+v", d.Attributions)
	}
}

func TestDiff_MicroDrift(t *testing.T) {
	base := core.NewBundle(core.Metadata{})
	base.AddModule(core.Module{
		ID:        "src/util.ts",
		SizeBytes: 1000,
	})

	current := core.NewBundle(core.Metadata{})
	current.AddModule(core.Module{
		ID:        "src/util.ts",
		SizeBytes: 1200, // +200 bytes (< 1000 drift threshold)
	})

	d := diff.Calculate(base, current, diff.Options{
		DriftThreshold: 1000,
	})

	if d.MicroDriftBytes != 200 {
		t.Fatalf("expected 200 bytes in micro-drift, got %d", d.MicroDriftBytes)
	}
}

func TestDiffTotalBytesCountsJavaScriptChunksOnly(t *testing.T) {
	b := core.NewBundle(core.Metadata{})
	b.AddEntrypoint("main", core.Entrypoint{InitialBytes: 120, AsyncBytes: 80})
	b.AddChunk(core.Chunk{ID: "main.js", SizeBytes: 100, Type: core.LoadTypeInitial})
	b.AddChunk(core.Chunk{ID: "lazy.js", SizeBytes: 200, Type: core.LoadTypeAsync})
	b.AddAsset(core.Asset{Path: "styles.css", SizeBytes: 20, MimeType: "text/css"})

	d := diff.Calculate(nil, b, diff.Options{})
	if d.Summary.HeadTotalBytes != 300 {
		t.Fatalf("total JS bytes = %d, want 300", d.Summary.HeadTotalBytes)
	}
}
