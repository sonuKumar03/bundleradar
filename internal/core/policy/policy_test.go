package policy_test

import (
	"testing"

	"github.com/sonuKumar03/bundleradar/internal/core"
	"github.com/sonuKumar03/bundleradar/internal/core/diff"
	"github.com/sonuKumar03/bundleradar/internal/core/policy"
)

func TestPolicy_LimitsAndRules(t *testing.T) {
	bundle := core.NewBundle(core.Metadata{})
	bundle.AddEntrypoint("main", core.Entrypoint{
		Name:         "main",
		InitialBytes: 250000,
	})
	bundle.AddModule(core.Module{
		ID:        "node_modules/moment/moment.js",
		Package:   "moment",
		Version:   "2.29.4",
		SizeBytes: 70000,
	})
	bundle.AddModule(core.Module{
		ID:        "node_modules/rxjs/index.js",
		Package:   "rxjs",
		Version:   "7.8.1",
		SizeBytes: 30000,
	})
	bundle.AddModule(core.Module{
		ID:        "node_modules/legacy-lib/node_modules/rxjs/index.js",
		Package:   "rxjs",
		Version:   "6.6.7", // Duplicate version of rxjs!
		SizeBytes: 28000,
	})

	bundleDiff := &diff.BundleDiff{
		Entrypoints: map[string]diff.EntrypointDelta{
			"main": {
				Name:         "main",
				InitialDelta: 15000, // +15KB
			},
		},
	}

	maxInitial := int64(200000) // 200KB limit (bundle is 250KB -> fails)
	maxInitialDelta := int64(10000) // 10KB delta limit (delta is 15KB -> fails)

	p := policy.Policy{
		MaxInitial:          &maxInitial,
		MaxInitialDelta:      &maxInitialDelta,
		ForbiddenPkgs:        []string{"moment"},
		DetectDuplicatePkgs: true,
	}

	res := policy.Evaluate(bundle, bundleDiff, p)
	if res.Passed {
		t.Fatalf("expected policy evaluation to fail, but it passed")
	}

	expectedRules := map[string]bool{
		"MAX_INITIAL_SIZE":        false,
		"MAX_INITIAL_DELTA":       false,
		"FORBIDDEN_PACKAGE":       false,
		"DUPLICATE_PACKAGE_VER":   false,
	}

	for _, v := range res.Violations {
		if _, ok := expectedRules[v.Rule]; ok {
			expectedRules[v.Rule] = true
		}
	}

	for rule, matched := range expectedRules {
		if !matched {
			t.Errorf("expected violation rule %q to be triggered, but was not. Violations: %+v", rule, res.Violations)
		}
	}
}

func TestPolicy_AllPassing(t *testing.T) {
	bundle := core.NewBundle(core.Metadata{})
	bundle.AddEntrypoint("main", core.Entrypoint{
		Name:         "main",
		InitialBytes: 100000,
	})

	maxInitial := int64(200000)
	p := policy.Policy{
		MaxInitial: &maxInitial,
	}

	res := policy.Evaluate(bundle, nil, p)
	if !res.Passed {
		t.Fatalf("expected policy to pass, but failed: %+v", res.Violations)
	}
}

func TestPolicy_InitialDeltaRequiresDiff(t *testing.T) {
	bundle := core.NewBundle(core.Metadata{})
	limit := int64(0)
	res := policy.Evaluate(bundle, nil, policy.Policy{MaxInitialDelta: &limit})
	if res.Passed || len(res.Violations) != 1 || res.Violations[0].Rule != "BASELINE_REQUIRED" {
		t.Fatalf("missing baseline result = %+v, want failed BASELINE_REQUIRED", res)
	}
}

func TestPolicy_LazyAndTotalDeltaBudgets(t *testing.T) {
	bundle := core.NewBundle(core.Metadata{})
	bundle.AddChunk(core.Chunk{ID: "lazy.js", SizeBytes: 200, Type: core.LoadTypeAsync})
	lazyLimit, totalDeltaLimit := int64(199), int64(9)
	res := policy.Evaluate(bundle, &diff.BundleDiff{Summary: diff.DiffSummary{TotalDeltaBytes: 10}}, policy.Policy{
		MaxLazy:       &lazyLimit,
		MaxTotalDelta: &totalDeltaLimit,
	})
	if res.Passed || len(res.Violations) != 2 {
		t.Fatalf("budget evaluation = %+v, want lazy and total-delta violations", res)
	}
	got := map[string]bool{}
	for _, v := range res.Violations {
		got[v.Rule] = true
	}
	if !got["MAX_LAZY_SIZE"] || !got["MAX_TOTAL_DELTA"] {
		t.Fatalf("budget violations = %+v, want MAX_LAZY_SIZE and MAX_TOTAL_DELTA", res.Violations)
	}
}
