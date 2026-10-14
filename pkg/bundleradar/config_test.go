package bundleradar_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sonuKumar03/bundleradar/pkg/bundleradar"
)

func TestToPolicy_CSSAndAssetBudgets(t *testing.T) {
	cfg := &bundleradar.Config{
		Budgets: bundleradar.ConfigBudgets{
			MaxCSS:    "100KB",
			MaxAssets: "2MB",
		},
	}
	pol, err := cfg.ToPolicy()
	if err != nil {
		t.Fatalf("ToPolicy: %v", err)
	}
	if pol.MaxCSS == nil || *pol.MaxCSS != 102400 {
		t.Fatalf("MaxCSS = %v, want 102400", pol.MaxCSS)
	}
	if pol.MaxAssets == nil || *pol.MaxAssets != 2*1024*1024 {
		t.Fatalf("MaxAssets = %v, want %d", pol.MaxAssets, 2*1024*1024)
	}
}

func TestToPolicy_MissingAssetBudgetsAreNil(t *testing.T) {
	pol, err := (&bundleradar.Config{}).ToPolicy()
	if err != nil {
		t.Fatalf("ToPolicy: %v", err)
	}
	if pol.MaxCSS != nil || pol.MaxAssets != nil {
		t.Fatalf("expected nil asset budgets, got MaxCSS=%v MaxAssets=%v", pol.MaxCSS, pol.MaxAssets)
	}
}
func TestConfig_ScanDefaultsAndExtraBudgets(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".bundleradar.yml")
	yml := "scan:\n  bundler: angular\n  dist: dist/browser\n  entry: src/main.ts\nbudgets:\n  max_lazy: 2mb\n  max_total_delta: 10kb\nrules:\n  detect_duplicate_packages: false\n"
	if err := os.WriteFile(path, []byte(yml), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := bundleradar.LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Scan.Bundler != "angular" || cfg.Scan.Entry != "src/main.ts" {
		t.Fatalf("scan = %+v", cfg.Scan)
	}
	if got := bundleradar.ResolvePath(path, cfg.Scan.Dist); got != filepath.Join(filepath.Dir(path), "dist/browser") {
		t.Fatalf("resolved dist = %q", got)
	}
	pol, err := cfg.ToPolicy()
	if err != nil {
		t.Fatal(err)
	}
	if pol.MaxLazy == nil || *pol.MaxLazy != 2*1024*1024 || pol.MaxTotalDelta == nil || pol.DetectDuplicatePkgs {
		t.Fatalf("policy = %+v", pol)
	}
}
