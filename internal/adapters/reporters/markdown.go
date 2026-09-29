package reporters

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/sonuKumar03/bundleradar/internal/core"
	"github.com/sonuKumar03/bundleradar/internal/core/diff"
	"github.com/sonuKumar03/bundleradar/internal/core/policy"
)

type MarkdownReporter struct{}

func (r *MarkdownReporter) Format() string {
	return "markdown"
}

func (r *MarkdownReporter) Render(ctx context.Context, w io.Writer, data any) error {
	var sb strings.Builder

	switch v := data.(type) {
	case *core.Bundle:
		sb.WriteString("## ⚡ BundleRadar Bundle Scan\n\n")
		sb.WriteString("| Entrypoint | Initial JS | Initial Gzip | Async JS | Chunks |\n")
		sb.WriteString("| :--- | :--- | :--- | :--- | :---: |\n")
		for name, ep := range v.Entrypoints {
			fmt.Fprintf(&sb, "| **`%s`** | `%s` | `%s` | `%s` | %d |\n",
				name, formatBytes(ep.InitialBytes), formatBytes(ep.InitialGzipBytes), formatBytes(ep.AsyncBytes), len(ep.ChunkIDs))
		}
		sb.WriteString("\n")

		top := v.TopPackages(10)
		if len(top) > 0 {
			sb.WriteString("### 📦 Top Contributing NPM Packages\n\n")
			sb.WriteString("| Package | Size | Est. Gzip | Modules |\n")
			sb.WriteString("| :--- | :--- | :--- | :---: |\n")
			for _, pkg := range top {
				fmt.Fprintf(&sb, "| `%s` | `%s` | `%s` | %d |\n",
					pkg.Name, formatBytes(pkg.SizeBytes), formatBytes(pkg.GzipBytes), pkg.ModuleCount)
			}
			sb.WriteString("\n")
		}

	case *diff.BundleDiff:
		sb.WriteString("## ⚡ BundleRadar Comparison & Diff\n\n")

		statusIcon := func(d int64) string {
			if d > 0 {
				return "⚠️ Increased"
			} else if d < 0 {
				return "🟢 Decreased"
			}
			return "⚪ Neutral"
		}
		formatDelta := func(d int64) string {
			if d > 0 {
				return "**+" + formatBytes(d) + "**"
			} else if d < 0 {
				return "**" + formatBytes(d) + "**"
			}
			return "**0 B**"
		}

		s := v.Summary
		if s.HeadTotalBytes > 0 || s.BaseTotalBytes > 0 {
			sb.WriteString("| Category | Before | After | Delta | Status |\n")
			sb.WriteString("| :--- | :---: | :---: | :---: | :---: |\n")
			fmt.Fprintf(&sb, "| **Initial JS** | `%s` | `%s` | %s | %s |\n",
				formatBytes(s.BaseInitialBytes), formatBytes(s.HeadInitialBytes), formatDelta(s.InitialDeltaBytes), statusIcon(s.InitialDeltaBytes))
			fmt.Fprintf(&sb, "| **Lazy JS** | `%s` | `%s` | %s | %s |\n",
				formatBytes(s.BaseLazyBytes), formatBytes(s.HeadLazyBytes), formatDelta(s.LazyDeltaBytes), statusIcon(s.LazyDeltaBytes))
			fmt.Fprintf(&sb, "| **Total JS** | `%s` | `%s` | %s | %s |\n\n",
				formatBytes(s.BaseTotalBytes), formatBytes(s.HeadTotalBytes), formatDelta(s.TotalDeltaBytes), statusIcon(s.TotalDeltaBytes))
		} else {
			sb.WriteString("| Entrypoint | Initial Delta | Gzip Delta | Async Delta |\n")
			sb.WriteString("| :--- | :--- | :--- | :--- |\n")
			for name, ep := range v.Entrypoints {
				initSign := "+"
				if ep.InitialDelta < 0 {
					initSign = ""
				}
				fmt.Fprintf(&sb, "| **`%s`** | `%s%s` | `+%s` | `+%s` |\n",
					name, initSign, formatBytes(ep.InitialDelta), formatBytes(ep.InitialGzipDelta), formatBytes(ep.AsyncDelta))
			}
			sb.WriteString("\n")
		}

		// Regression Explanations
		var majorRegressions []diff.PackageDelta
		var minorVariations []diff.PackageDelta
		for _, pkg := range v.Packages {
			if pkg.DeltaBytes >= 1024 || (pkg.BaseBytes == 0 && pkg.DeltaBytes > 0) {
				majorRegressions = append(majorRegressions, pkg)
			} else if pkg.DeltaBytes > 0 {
				minorVariations = append(minorVariations, pkg)
			}
		}

		if len(majorRegressions) > 0 {
			sb.WriteString("### 🔎 Regression Explanation\n\n")
			for _, reg := range majorRegressions {
				chunkDesc := ""
				if len(reg.ChunkNames) > 0 {
					chunkDesc = fmt.Sprintf(" → emitted in `%s`", strings.Join(reg.ChunkNames, "`, `"))
				}
				fmt.Fprintf(&sb, "- 📦 **`%s`** (`+%s`)%s\n", reg.Name, formatBytes(reg.DeltaBytes), chunkDesc)
				if reg.ImportPath != "" {
					fmt.Fprintf(&sb, "  - **Import path:** `%s`\n", reg.ImportPath)
				}
			}
			sb.WriteString("\n")
		}

		if len(minorVariations) > 0 {
			fmt.Fprintf(&sb, "<details>\n<summary>⚪ %d Minor Variations (&lt; 1 KB)</summary>\n\n", len(minorVariations))
			for _, m := range minorVariations {
				chunkDesc := ""
				if len(m.ChunkNames) > 0 {
					chunkDesc = fmt.Sprintf(" → emitted in `%s`", strings.Join(m.ChunkNames, "`, `"))
				}
				fmt.Fprintf(&sb, "- 📦 **`%s`** (`+%s`)%s\n", m.Name, formatBytes(m.DeltaBytes), chunkDesc)
				if m.ImportPath != "" {
					fmt.Fprintf(&sb, "  - **Import path:** `%s`\n", m.ImportPath)
				}
			}
			sb.WriteString("\n</details>\n\n")
		}

		// Changed Packages Table
		if len(v.Packages) > 0 {
			sb.WriteString("### Changed Packages\n\n")
			sb.WriteString("| Package | Status | Delta | Base Size | Current Size |\n")
			sb.WriteString("| :--- | :---: | :---: | :---: | :---: |\n")
			for _, pkg := range v.Packages {
				status := "🔄 Changed"
				if pkg.BaseBytes == 0 {
					status = "➕ Added"
				} else if pkg.CurrBytes == 0 {
					status = "➖ Removed"
				}
				sign := "+"
				if pkg.DeltaBytes < 0 {
					sign = ""
				}
				fmt.Fprintf(&sb, "| `%s` | %s | `%s%s` | `%s` | `%s` |\n",
					pkg.Name, status, sign, formatBytes(pkg.DeltaBytes), formatBytes(pkg.BaseBytes), formatBytes(pkg.CurrBytes))
			}
			sb.WriteString("\n")
		}

		// Collapsible Unchanged Packages & Micro-Drift
		if len(v.UnchangedPackages) > 0 || v.MicroDriftBytes > 0 {
			summaryText := fmt.Sprintf("⚪ %d Unchanged Packages", len(v.UnchangedPackages))
			if v.MicroDriftBytes > 0 {
				summaryText += fmt.Sprintf(" & Micro-drift (%s)", formatBytes(v.MicroDriftBytes))
			}
			fmt.Fprintf(&sb, "<details>\n<summary>%s</summary>\n\n", summaryText)
			if v.MicroDriftBytes > 0 {
				fmt.Fprintf(&sb, "> ℹ️ **Micro-drift**: %s collapsed across sub-threshold module updates.\n\n", formatBytes(v.MicroDriftBytes))
			}
			if len(v.UnchangedPackages) > 0 {
				sb.WriteString("| Package | Status | Size |\n")
				sb.WriteString("| :--- | :---: | :---: |\n")
				for _, pkg := range v.UnchangedPackages {
					fmt.Fprintf(&sb, "| `%s` | ⚪ Unchanged | `%s` |\n", pkg.Name, formatBytes(pkg.CurrBytes))
				}
				sb.WriteString("\n")
			}
			sb.WriteString("</details>\n\n")
		}

	case policy.EvaluationResult:
		if v.Passed {
			sb.WriteString("## ⚡ BundleRadar Gate: ✅ All Size Budgets Passed\n\n")
			sb.WriteString("All bundle size limits and architectural policies are satisfied.\n\n")
		} else {
			sb.WriteString("## ⚡ BundleRadar Gate: ❌ Policy Check Failed\n\n")
			sb.WriteString("| Rule | Severity | Violation Message |\n")
			sb.WriteString("| :--- | :--- | :--- |\n")
			for _, viol := range v.Violations {
				fmt.Fprintf(&sb, "| **`%s`** | `%s` | %s |\n", viol.Rule, viol.Severity, viol.Message)
			}
			sb.WriteString("\n")
		}
		for _, warning := range v.Warnings {
			fmt.Fprintf(&sb, "- **Warning `%s`**: %s\n", warning.Rule, warning.Message)
		}
		if len(v.Warnings) > 0 {
			sb.WriteString("\n")
		}

	default:
		return fmt.Errorf("markdown reporter: unsupported data type %T", data)
	}

	_, err := io.WriteString(w, sb.String())
	return err
}
