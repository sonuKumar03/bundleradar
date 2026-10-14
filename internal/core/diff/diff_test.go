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
		DriftThreshold:   1000, // 1KB
		IncludeUnchanged: true,
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

func TestDiff_UnchangedPackagesAreOptIn(t *testing.T) {
	mk := func() *core.Bundle {
		b := core.NewBundle(core.Metadata{})
		b.AddChunk(core.Chunk{ID: "main.js", SizeBytes: 100, Type: core.LoadTypeInitial})
		b.AddModule(core.Module{ID: "node_modules/lodash/a.js", Package: "lodash", SizeBytes: 100, ChunkIDs: []string{"main.js"}, ChunkBytes: map[string]int64{"main.js": 100}})
		return b
	}
	d := diff.Calculate(mk(), mk(), diff.Options{})
	if len(d.UnchangedPackages) != 0 || d.UnchangedCount != 1 {
		t.Fatalf("unchanged list=%d count=%d, want 0 and 1", len(d.UnchangedPackages), d.UnchangedCount)
	}
}

func TestDiff_LazyToInitialPromotionIsReported(t *testing.T) {
	base := core.NewBundle(core.Metadata{})
	base.AddChunk(core.Chunk{ID: "main.js", SizeBytes: 100, Type: core.LoadTypeInitial})
	base.AddChunk(core.Chunk{ID: "lazy.js", SizeBytes: 500, Type: core.LoadTypeAsync})
	base.AddModule(core.Module{ID: "node_modules/pdfjs/a.js", Package: "pdfjs", SizeBytes: 500, ChunkIDs: []string{"lazy.js"}, ChunkBytes: map[string]int64{"lazy.js": 500}, IngressPaths: []string{"src/main.ts", "node_modules/pdfjs/a.js"}})
	head := core.NewBundle(core.Metadata{})
	head.AddChunk(core.Chunk{ID: "main.js", SizeBytes: 600, Type: core.LoadTypeInitial})
	head.AddModule(core.Module{ID: "node_modules/pdfjs/a.js", Package: "pdfjs", SizeBytes: 500, ChunkIDs: []string{"main.js"}, ChunkBytes: map[string]int64{"main.js": 500}, IngressPaths: []string{"src/main.ts", "node_modules/pdfjs/a.js"}})

	d := diff.Calculate(base, head, diff.Options{})
	if len(d.Packages) != 1 || d.Packages[0].Status != diff.StatusPromoted || d.Packages[0].InitialDelta != 500 {
		t.Fatalf("packages = %+v, want pdfjs PROMOTED with initial delta 500", d.Packages)
	}
	if len(d.Attributions) != 1 || d.Attributions[0].DeltaBytes != 500 {
		t.Fatalf("attributions = %+v, want one 500B regression", d.Attributions)
	}
}

func TestDiff_FirstPartyChangesAreGrouped(t *testing.T) {
	mk := func(size int64) *core.Bundle {
		b := core.NewBundle(core.Metadata{})
		b.AddChunk(core.Chunk{ID: "main.js", SizeBytes: size, Type: core.LoadTypeInitial})
		b.AddModule(core.Module{ID: "libs/pdf-sign/src/a.ts", IsAppCode: true, SizeBytes: size, ChunkIDs: []string{"main.js"}, ChunkBytes: map[string]int64{"main.js": size}})
		return b
	}
	d := diff.Calculate(mk(100), mk(400), diff.Options{})
	if len(d.FirstParty) != 1 || d.FirstParty[0].Name != "libs/pdf-sign" || d.FirstParty[0].DeltaBytes != 300 {
		t.Fatalf("firstParty = %+v, want libs/pdf-sign +300", d.FirstParty)
	}
}

func TestDiff_HashedChunkRenameIsAChange(t *testing.T) {
	base := core.NewBundle(core.Metadata{})
	base.AddChunk(core.Chunk{ID: "main-AAAAAAAA.js", Name: "main-AAAAAAAA.js", SizeBytes: 100, ModuleIDs: []string{"a", "b"}})
	base.AddChunk(core.Chunk{ID: "chunk-11111111.js", Name: "chunk-11111111.js", SizeBytes: 50, ModuleIDs: []string{"x", "y", "z"}})
	head := core.NewBundle(core.Metadata{})
	head.AddChunk(core.Chunk{ID: "main-BBBBBBBB.js", Name: "main-BBBBBBBB.js", SizeBytes: 130, ModuleIDs: []string{"a", "b"}})
	head.AddChunk(core.Chunk{ID: "chunk-22222222.js", Name: "chunk-22222222.js", SizeBytes: 60, ModuleIDs: []string{"x", "y", "z", "w"}})

	d := diff.Calculate(base, head, diff.Options{})
	if len(d.AddedChunks) != 0 || len(d.RemovedChunks) != 0 {
		t.Fatalf("added=%d removed=%d, want none", len(d.AddedChunks), len(d.RemovedChunks))
	}
	if len(d.ChangedChunks) != 2 || d.ChangedChunks[0].Name != "main.js" || d.ChangedChunks[0].DeltaBytes != 30 {
		t.Fatalf("changed = %+v", d.ChangedChunks)
	}
}
