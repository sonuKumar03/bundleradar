package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sonuKumar03/bundleradar/internal/adapters/parsers"
	"github.com/sonuKumar03/bundleradar/internal/core"
)

// requireBuiltStats returns the stats path for a fixture app built by a real
// bundler. Tests skip when the build output is not present (local unit runs)
// but fail when BUNDLECHECK_REQUIRE_E2E is set, so the CI e2e job cannot
// silently degrade to skipped tests.
func requireBuiltStats(t *testing.T, fixture, statsRel string) string {
	t.Helper()

	const envKey = "BUNDLECHECK_REQUIRE_E2E"
	statsPath := filepath.Join("..", "testdata", fixture, statsRel)
	if _, err := os.Stat(statsPath); os.IsNotExist(err) {
		if os.Getenv(envKey) != "" {
			t.Fatalf("%s is set but built stats missing at %s; run the fixture build first", envKey, statsPath)
		}
		t.Skipf("skipping: %s build output not present at %s (build the fixture to enable)", fixture, statsPath)
	}
	return statsPath
}

func mustEntrypoint(t *testing.T, bundle *core.Bundle, name string) core.Entrypoint {
	t.Helper()
	ep, ok := bundle.Entrypoints[name]
	if !ok {
		t.Fatalf("missing %q entrypoint; got %v", name, entrypointNames(bundle))
	}
	return ep
}

func entrypointNames(bundle *core.Bundle) []string {
	names := make([]string, 0, len(bundle.Entrypoints))
	for name := range bundle.Entrypoints {
		names = append(names, name)
	}
	return names
}

func TestViteApp_RealBuildInvariants(t *testing.T) {
	statsPath := requireBuiltStats(t, "vite-app", filepath.Join("dist", ".vite", "manifest.json"))

	parser := &parsers.ViteParser{}
	bundle, err := parser.Parse(t.Context(), core.Target{
		Name:      "vite-app",
		StatsPath: statsPath,
		DistPath:  filepath.Join("..", "testdata", "vite-app", "dist"),
	})
	if err != nil {
		t.Fatalf("parse vite manifest: %v", err)
	}

	ep := mustEntrypoint(t, bundle, "index.html")
	if ep.InitialBytes <= 0 {
		t.Errorf("vite entry initial bytes must be positive, got %d", ep.InitialBytes)
	}

	// The app has a dynamic import; entrypoint async bytes must reflect it.
	if ep.AsyncBytes <= 0 {
		t.Errorf("vite entry with dynamic import reports 0 async bytes; dynamicImports attribution broken")
	}

	var initialChunks, asyncChunks int
	var asyncBytes int64
	for _, ch := range bundle.Chunks {
		switch ch.Type {
		case core.LoadTypeInitial:
			initialChunks++
		case core.LoadTypeAsync:
			asyncChunks++
			asyncBytes += ch.SizeBytes
		}
	}
	if initialChunks != 1 || asyncChunks != 1 {
		t.Errorf("expected 1 initial + 1 async chunk in real vite build, got %d initial + %d async", initialChunks, asyncChunks)
	}
	if asyncBytes != ep.AsyncBytes {
		t.Errorf("entrypoint async bytes (%d) != sum of async chunks (%d)", ep.AsyncBytes, asyncBytes)
	}

	// Sizes must come from the real emitted files on disk.
	for _, ch := range bundle.Chunks {
		if ch.SizeBytes <= 0 {
			t.Errorf("vite chunk %q has no size; dist file stat failed", ch.ID)
		}
	}

	// Vite manifests carry no module-level info: dependencies are bundled
	// inside entry chunks, so per-package attribution is structurally
	// impossible from a manifest alone (app code covers everything).
	pkgs := packageBytes(bundle)
	for name := range pkgs {
		if name != "" && name != "(application code)" {
			t.Errorf("vite manifest attributed package %q, but manifests cannot express package provenance; parser is inventing attribution", name)
		}
	}
	if bundle.TotalAppCodeBytes() <= 0 {
		t.Errorf("vite build must report application-code bytes")
	}
}

func TestWebpackApp_RealBuildInvariants(t *testing.T) {
	statsPath := requireBuiltStats(t, "webpack-app", filepath.Join("dist", "stats.json"))

	parser := &parsers.WebpackParser{}
	bundle, err := parser.Parse(t.Context(), core.Target{
		Name:      "webpack-app",
		StatsPath: statsPath,
	})
	if err != nil {
		t.Fatalf("parse webpack stats: %v", err)
	}

	// Single-entry app: every non-initial chunk is unambiguously lazy.
	ep := mustEntrypoint(t, bundle, "main")
	if ep.InitialBytes <= 0 {
		t.Errorf("webpack entry initial bytes must be positive, got %d", ep.InitialBytes)
	}
	if ep.AsyncBytes <= 0 {
		t.Errorf("webpack entry with dynamic import reports 0 async bytes; single-entry async attribution broken")
	}

	// No chunk may keep the "<nil>" placeholder ID that bare stats structs
	// produce when webpack omits id/names for optimization-generated chunks.
	for _, ch := range bundle.Chunks {
		if ch.ID == "" || ch.ID == "<nil>" {
			t.Errorf("webpack chunk has placeholder ID %q; display-name fallback broken", ch.ID)
		}
	}

	// Aggregate rollups ("runtime modules", "dependent modules") must not
	// become phantom modules, and lodash must be attributed.
	pkgs := packageBytes(bundle)
	if _, ok := pkgs["lodash"]; !ok {
		t.Errorf("expected lodash attributed in real webpack build; got packages %v", pkgs)
	}
	var phantomModules int
	for _, m := range bundle.Modules {
		if m.ID == "" {
			phantomModules++
		}
	}
	if phantomModules != 0 {
		t.Errorf("%d nameless phantom modules leaked from webpack aggregate rollups", phantomModules)
	}

	var appModules int
	for _, m := range bundle.Modules {
		if m.IsAppCode {
			appModules++
		}
	}
	if appModules == 0 {
		t.Errorf("no application-code modules detected in real webpack build")
	}
}
