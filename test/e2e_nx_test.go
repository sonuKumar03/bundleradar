package test

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/sonuKumar03/bundleradar/internal/adapters/parsers"
	"github.com/sonuKumar03/bundleradar/internal/core"
)

// nxStatsPath returns the stats path for an application built by the real Nx
// workspace in testdata/nx-workspace. Tests skip when the build output is not
// present (local unit runs) but fail when BUNDLECHECK_REQUIRE_E2E is set, so
// the CI e2e job cannot silently degrade to skipped tests.
func nxStatsPath(t *testing.T, app string) string {
	t.Helper()

	const envKey = "BUNDLECHECK_REQUIRE_E2E"
	statsPath := filepath.Join("..", "testdata", "nx-workspace", "dist", "apps", app, "stats.json")
	if _, err := os.Stat(statsPath); os.IsNotExist(err) {
		if os.Getenv(envKey) != "" {
			t.Fatalf("%s is set but built stats missing at %s; run `npm run build` in testdata/nx-workspace first", envKey, statsPath)
		}
		t.Skipf("skipping: %s stats not present at %s (build the Nx workspace to enable)", app, statsPath)
	}
	return statsPath
}

func parseNxStats(t *testing.T, app string) *core.Bundle {
	t.Helper()

	parser := &parsers.AngularParser{}
	bundle, err := parser.Parse(t.Context(), core.Target{
		Name:      app,
		StatsPath: nxStatsPath(t, app),
	})
	if err != nil {
		t.Fatalf("parse %s stats: %v", app, err)
	}
	return bundle
}

func packageBytes(bundle *core.Bundle) map[string]int64 {
	pkgs := make(map[string]int64)
	for _, m := range bundle.Modules {
		if m.Package == "" {
			continue
		}
		pkgs[m.Package] += m.SizeBytes
	}
	return pkgs
}

// TestNxWorkspace_PortalInvariants asserts semantic properties of the real
// portal build. Invariants (not byte snapshots) so Angular patch releases that
// shift hashes/sizes slightly don't create noise, while parser regressions do.
func TestNxWorkspace_PortalInvariants(t *testing.T) {
	bundle := parseNxStats(t, "portal")

	main, ok := bundle.Entrypoints["main"]
	if !ok {
		t.Fatalf("portal bundle missing 'main' entrypoint; got %d entrypoints", len(bundle.Entrypoints))
	}
	if main.InitialBytes <= 0 {
		t.Errorf("portal initial bytes must be positive, got %d", main.InitialBytes)
	}

	// Portal declares lazy routes; the real build must classify them as async.
	if main.AsyncBytes <= 0 {
		t.Errorf("portal has lazy routes but async bytes = 0; initial/async classification is broken")
	}

	// Entrance chunk bytes plus separately-tracked initial CSS should
	// approximate the entrypoint total; allow slack for assets counted
	// outside the JS chunk set.
	var initialChunkBytes int64
	var asyncChunkCount int
	for _, ch := range bundle.Chunks {
		switch ch.Type {
		case core.LoadTypeInitial:
			initialChunkBytes += ch.SizeBytes
		case core.LoadTypeAsync:
			asyncChunkCount++
		}
	}
	if main.InitialBytes < initialChunkBytes || main.InitialBytes > initialChunkBytes*2 {
		t.Errorf("entrypoint initial bytes (%d) far from sum of initial chunks (%d); attribution drifted", main.InitialBytes, initialChunkBytes)
	}
	if asyncChunkCount == 0 {
		t.Errorf("portal build produced 0 async chunks despite lazy routes")
	}

	pkgs := packageBytes(bundle)
	for _, want := range []string{"moment", "lodash", "chart.js", "d3", "@angular/core", "@angular/router"} {
		if _, ok := pkgs[want]; !ok {
			t.Errorf("expected package %q in real portal bundle; missing from attribution", want)
		}
	}
	if _, ok := pkgs["lodash-es"]; ok {
		t.Errorf("lodash-es should be tree-shaken out of the bundle (only lodash imported), but was attributed")
	}

	var appModules int
	for _, m := range bundle.Modules {
		if m.IsAppCode {
			appModules++
		}
	}
	if appModules == 0 {
		t.Errorf("no application-code modules detected in real portal build")
	}
	if bundle.TotalAppCodeBytes() <= 0 {
		t.Errorf("application code bytes must be positive, got %d", bundle.TotalAppCodeBytes())
	}
}

// TestNxWorkspace_AdminDashboardInvariants asserts the second real application
// remains scannable and materially smaller than the portal app.
func TestNxWorkspace_AdminDashboardInvariants(t *testing.T) {
	admin := parseNxStats(t, "admin-dashboard")
	portal := parseNxStats(t, "portal")

	adminEP, ok := admin.Entrypoints["main"]
	if !ok {
		t.Fatalf("admin-dashboard bundle missing 'main' entrypoint")
	}
	if adminEP.InitialBytes <= 0 {
		t.Errorf("admin-dashboard initial bytes must be positive, got %d", adminEP.InitialBytes)
	}
	portalEP := portal.Entrypoints["main"]
	if portalEP.InitialBytes <= adminEP.InitialBytes {
		t.Errorf("expected portal initial (%d) > admin-dashboard initial (%d)", portalEP.InitialBytes, adminEP.InitialBytes)
	}
}

// TestNxWorkspace_FixtureDrift guards the committed stats fixtures
// (apps/<app>/stats.json) against silently diverging from what the current
// sources actually produce (dist/apps/<app>/stats.json). Package sets must
// match exactly; byte totals within 5% to tolerate version drift.
func TestNxWorkspace_FixtureDrift(t *testing.T) {
	for _, app := range []string{"portal", "admin-dashboard"} {
		t.Run(app, func(t *testing.T) {
			committedPath := filepath.Join("..", "testdata", "nx-workspace", "apps", app, "stats.json")
			if _, err := os.Stat(committedPath); os.IsNotExist(err) {
				t.Fatalf("committed fixture stats missing at %s", committedPath)
			}

			parser := &parsers.AngularParser{}
			committed, err := parser.Parse(t.Context(), core.Target{StatsPath: committedPath})
			if err != nil {
				t.Fatalf("parse committed %s stats: %v", app, err)
			}
			fresh := parseNxStats(t, app)

			committedPkgs := packageBytes(committed)
			freshPkgs := packageBytes(fresh)

			for name := range committedPkgs {
				if _, ok := freshPkgs[name]; !ok {
					t.Errorf("committed fixture has package %q but fresh build does not; fixture is stale", name)
				}
			}
			for name := range freshPkgs {
				if _, ok := committedPkgs[name]; !ok {
					t.Errorf("fresh build produces package %q but committed fixture lacks it; fixture is stale", name)
				}
			}

			compareTotal := func(label string, committedTotal, freshTotal int64) {
				if committedTotal == 0 && freshTotal == 0 {
					return
				}
				delta := math.Abs(float64(freshTotal - committedTotal))
				base := math.Max(float64(committedTotal), float64(freshTotal))
				if delta/base > 0.05 {
					t.Errorf("%s drifted beyond 5%%: committed %d vs fresh %d bytes; regenerate the fixture stats", label, committedTotal, freshTotal)
				}
			}
			compareTotal("initial bytes", committed.Entrypoints["main"].InitialBytes, fresh.Entrypoints["main"].InitialBytes)
			compareTotal("async bytes", committed.Entrypoints["main"].AsyncBytes, fresh.Entrypoints["main"].AsyncBytes)
			compareTotal("package bytes", sumValues(committedPkgs), sumValues(freshPkgs))
		})
	}
}

func sumValues(m map[string]int64) int64 {
	var total int64
	for _, v := range m {
		total += v
	}
	return total
}
