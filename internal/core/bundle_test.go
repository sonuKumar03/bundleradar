package core_test

import (
	"encoding/json"
	"testing"

	"github.com/sonuKumar03/bundleradar/internal/core"
)

func TestBundleAggregation(t *testing.T) {
	b := core.NewBundle(core.Metadata{
		Bundler: "esbuild",
	})

	b.AddEntrypoint("main", core.Entrypoint{
		Name:             "main",
		InitialBytes:     150000,
		InitialGzipBytes: 45000,
		AsyncBytes:       200000,
		ChunkIDs:         []string{"main.js", "styles.css"},
	})

	b.AddEntrypoint("admin", core.Entrypoint{
		Name:             "admin",
		InitialBytes:     80000,
		InitialGzipBytes: 24000,
		AsyncBytes:       50000,
		ChunkIDs:         []string{"admin.js"},
	})

	b.AddChunk(core.Chunk{
		ID:        "main.js",
		Name:      "main.js",
		Path:      "dist/main.js",
		SizeBytes: 130000,
		GzipBytes: 39000,
		Type:      core.LoadTypeInitial,
		Entry:     "main",
		ModuleIDs: []string{"src/main.ts", "node_modules/lodash-es/map.js"},
	})

	b.AddModule(core.Module{
		ID:        "node_modules/lodash-es/map.js",
		Package:   "lodash-es",
		Version:   "4.17.21",
		SizeBytes: 15000,
		GzipBytes: 4500,
		IsAppCode: false,
		ChunkIDs:  []string{"main.js"},
	})

	b.AddModule(core.Module{
		ID:        "src/main.ts",
		Package:   "",
		SizeBytes: 25000,
		GzipBytes: 7500,
		IsAppCode: true,
		ChunkIDs:  []string{"main.js"},
	})

	if b.TotalInitialBytes() != 230000 {
		t.Fatalf("expected 230000 total initial bytes, got %d", b.TotalInitialBytes())
	}

	if b.TotalAsyncBytes() != 250000 {
		t.Fatalf("expected 250000 total async bytes, got %d", b.TotalAsyncBytes())
	}

	topPkgs := b.TopPackages(5)
	if len(topPkgs) != 1 {
		t.Fatalf("expected 1 top package, got %d", len(topPkgs))
	}
	if topPkgs[0].Name != "lodash-es" || topPkgs[0].SizeBytes != 15000 {
		t.Fatalf("unexpected top package: %+v", topPkgs[0])
	}

	appBytes := b.TotalAppCodeBytes()
	if appBytes != 25000 {
		t.Fatalf("expected 25000 app code bytes, got %d", appBytes)
	}

	chunk, found := b.FindChunk("main.js")
	if !found || chunk.SizeBytes != 130000 {
		t.Fatalf("failed to find chunk main.js")
	}

	mod, found := b.FindModule("src/main.ts")
	if !found || !mod.IsAppCode {
		t.Fatalf("failed to find module src/main.ts")
	}
}

func TestBundleResolveEntrypoint(t *testing.T) {
	b := core.NewBundle(core.Metadata{Bundler: "angular"})
	b.AddEntrypoint("main", core.Entrypoint{
		Name:             "main",
		InitialBytes:     50000,
		InitialGzipBytes: 15000,
		ChunkIDs:         []string{"main.js", "polyfills.js"},
	})
	b.AddChunk(core.Chunk{
		ID:        "main.js",
		Name:      "main.js",
		Path:      "dist/browser/main.js",
		SizeBytes: 40000,
		Entry:     "src/main.ts",
		ModuleIDs: []string{"src/main.ts"},
	})
	b.AddChunk(core.Chunk{
		ID:        "polyfills.js",
		Name:      "polyfills.js",
		Path:      "dist/browser/polyfills.js",
		SizeBytes: 10000,
		Entry:     "src/polyfills.ts",
		ModuleIDs: []string{"src/polyfills.ts"},
	})

	tests := []struct {
		query string
		want  string
	}{
		{"main", "main"},
		{"MAIN", "main"},
		{"src/main.ts", "main"},
		{"./src/main.ts", "main"},
		{"main.js", "main"},
		{"main.ts", "main"},
		{"src/polyfills.ts", "main"},
		{"polyfills.js", "main"},
	}

	for _, tc := range tests {
		ep, ok := b.ResolveEntrypoint(tc.query)
		if !ok || ep == nil {
			t.Errorf("ResolveEntrypoint(%q) expected found, got false", tc.query)
			continue
		}
		if ep.Name != tc.want {
			t.Errorf("ResolveEntrypoint(%q) = %q, want %q", tc.query, ep.Name, tc.want)
		}
	}

	if _, ok := b.ResolveEntrypoint("non-existent"); ok {
		t.Errorf("ResolveEntrypoint(non-existent) expected false, got true")
	}
}

// TestBundleIndexLifecycleAcrossSerialization ensures AddChunk/AddModule and
// FindChunk/FindModule stay consistent when a Bundle is round-tripped through
// JSON (which drops the unexported indexes). Regression guard for nil-map
// panics and stale-index reads after deserialization.
func TestBundleIndexLifecycleAcrossSerialization(t *testing.T) {
	b := core.NewBundle(core.Metadata{Bundler: "esbuild"})
	b.AddChunk(core.Chunk{ID: "a.js", Name: "a.js"})
	b.AddModule(core.Module{ID: "src/a.ts"})

	data, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var restored core.Bundle
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Lookups must rebuild the index after deserialization.
	if _, ok := restored.FindChunk("a.js"); !ok {
		t.Errorf("FindChunk after deserialize failed")
	}
	if _, ok := restored.FindModule("src/a.ts"); !ok {
		t.Errorf("FindModule after deserialize failed")
	}

	// Mutating after deserialization (and after an index build) must not
	// panic or corrupt lookups for pre-existing entries.
	restored.AddChunk(core.Chunk{ID: "b.js", Name: "b.js"})
	restored.AddModule(core.Module{ID: "src/b.ts"})

	if _, ok := restored.FindChunk("a.js"); !ok {
		t.Errorf("FindChunk(a.js) lost after post-deserialize mutation")
	}
	if _, ok := restored.FindChunk("b.js"); !ok {
		t.Errorf("FindChunk(b.js) missing after post-deserialize mutation")
	}
	if _, ok := restored.FindModule("src/a.ts"); !ok {
		t.Errorf("FindModule(src/a.ts) lost after post-deserialize mutation")
	}
	if _, ok := restored.FindModule("src/b.ts"); !ok {
		t.Errorf("FindModule(src/b.ts) missing after post-deserialize mutation")
	}
}

