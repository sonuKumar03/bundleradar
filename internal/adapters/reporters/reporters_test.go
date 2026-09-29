package reporters_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/sonuKumar03/bundleradar/internal/adapters/reporters"
	"github.com/sonuKumar03/bundleradar/internal/core"
	"github.com/sonuKumar03/bundleradar/internal/core/diff"
	"github.com/sonuKumar03/bundleradar/internal/core/policy"
)

func TestReporters_MarkdownAndPR(t *testing.T) {
	bundle := core.NewBundle(core.Metadata{Bundler: "esbuild"})
	bundle.AddEntrypoint("main", core.Entrypoint{
		Name:             "main",
		InitialBytes:     150000,
		InitialGzipBytes: 45000,
		AsyncBytes:       200000,
	})

	mdRep, err := reporters.New("markdown")
	if err != nil {
		t.Fatalf("failed to create markdown reporter: %v", err)
	}

	var buf bytes.Buffer
	if err := mdRep.Render(context.Background(), &buf, bundle); err != nil {
		t.Fatalf("render markdown failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "## ⚡ BundleRadar Bundle Scan") {
		t.Fatalf("missing header in markdown report: %s", out)
	}
	if !strings.Contains(out, "| **`main`** |") {
		t.Fatalf("missing entrypoint in markdown table: %s", out)
	}

	// PR reporter must include sticky marker
	prRep, err := reporters.New("github-pr")
	if err != nil {
		t.Fatalf("failed to create pr reporter: %v", err)
	}
	buf.Reset()
	if err := prRep.Render(context.Background(), &buf, bundle); err != nil {
		t.Fatalf("render pr failed: %v", err)
	}
	if !strings.Contains(buf.String(), "<!-- bundleradar-report -->") {
		t.Fatalf("missing sticky marker in PR comment: %s", buf.String())
	}
}

func TestReporters_JSON(t *testing.T) {
	bundle := core.NewBundle(core.Metadata{Bundler: "vite"})
	bundle.AddEntrypoint("app", core.Entrypoint{
		Name:         "app",
		InitialBytes: 50000,
	})

	jsonRep, err := reporters.New("json")
	if err != nil {
		t.Fatalf("failed to create json reporter: %v", err)
	}

	var buf bytes.Buffer
	if err := jsonRep.Render(context.Background(), &buf, bundle); err != nil {
		t.Fatalf("render json failed: %v", err)
	}

	if !strings.Contains(buf.String(), `"entrypoints"`) || !strings.Contains(buf.String(), `"app"`) {
		t.Fatalf("unexpected json output: %s", buf.String())
	}
}

func TestReporters_GateResult(t *testing.T) {
	evalRes := policy.EvaluationResult{
		Passed: false,
		Violations: []policy.Violation{
			{
				Severity: "error",
				Rule:     "MAX_INITIAL_SIZE",
				Message:  "Initial bundle 300KB exceeds 250KB",
			},
		},
	}

	mdRep, _ := reporters.New("markdown")
	var buf bytes.Buffer
	if err := mdRep.Render(context.Background(), &buf, evalRes); err != nil {
		t.Fatalf("render gate result failed: %v", err)
	}

	if !strings.Contains(buf.String(), "❌ Policy Check Failed") {
		t.Fatalf("expected failure badge in gate report: %s", buf.String())
	}
}

func TestReporters_GateWarnings(t *testing.T) {
	evalRes := policy.EvaluationResult{
		Passed: true,
		Warnings: []policy.Violation{{
			Severity: "warning",
			Rule:     "DUPLICATE_PACKAGE_VERSION_UNKNOWN",
			Message:  "Version metadata is unavailable for 1 package.",
		}},
	}
	for _, format := range []string{"terminal", "markdown"} {
		reporter, err := reporters.New(format)
		if err != nil {
			t.Fatal(err)
		}
		var output bytes.Buffer
		if err := reporter.Render(context.Background(), &output, evalRes); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(output.String(), "DUPLICATE_PACKAGE_VERSION_UNKNOWN") {
			t.Fatalf("%s report omitted the policy warning: %s", format, output.String())
		}
	}
}

func TestReporters_DiffMarkdown(t *testing.T) {
	d := &diff.BundleDiff{
		Summary: diff.DiffSummary{
			BaseInitialBytes:  1000000,
			HeadInitialBytes:  1300000,
			InitialDeltaBytes: 300000,
			BaseLazyBytes:     50000,
			HeadLazyBytes:     50000,
			LazyDeltaBytes:    0,
			BaseTotalBytes:    1050000,
			HeadTotalBytes:    1350000,
			TotalDeltaBytes:   300000,
		},
		Packages: []diff.PackageDelta{
			{
				Name:       "three",
				DeltaBytes: 300000,
				BaseBytes:  0,
				CurrBytes:  300000,
				ChunkNames: []string{"main.js"},
				ImportPath: "src/main.ts → src/app.ts → three",
			},
		},
	}

	mdRep, _ := reporters.New("markdown")
	var buf bytes.Buffer
	if err := mdRep.Render(context.Background(), &buf, d); err != nil {
		t.Fatalf("render diff markdown failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "## ⚡ BundleRadar Comparison & Diff") {
		t.Fatalf("missing diff header: %s", out)
	}
	if !strings.Contains(out, "| **Initial JS** |") || !strings.Contains(out, "⚠️ Increased") {
		t.Fatalf("missing category summary table: %s", out)
	}
	if !strings.Contains(out, "### 🔎 Regression Explanation") || !strings.Contains(out, "src/main.ts → src/app.ts → three") {
		t.Fatalf("missing regression explanation or import path: %s", out)
	}
	if !strings.Contains(out, "### Changed Packages") || !strings.Contains(out, "➕ Added") {
		t.Fatalf("missing changed packages table: %s", out)
	}
}
