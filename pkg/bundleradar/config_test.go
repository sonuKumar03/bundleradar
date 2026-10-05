package bundleradar_test

import (
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