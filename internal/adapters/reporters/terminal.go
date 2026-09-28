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

type TerminalReporter struct {
	Top int
}

func (r *TerminalReporter) Format() string {
	return "terminal"
}

func (r *TerminalReporter) Render(ctx context.Context, w io.Writer, data any) error {
	var sb strings.Builder

	switch v := data.(type) {
	case *core.Bundle:
		sb.WriteString("\n⚡ BUNDLERADAR BUNDLE SCAN\n")
		sb.WriteString("-------------------------------------------------------------\n")
		for name, ep := range v.Entrypoints {
			fmt.Fprintf(&sb, "Entrypoint %q:\n", name)
			fmt.Fprintf(&sb, "  Initial JavaScript:   %s [%s%s gzip]\n", formatBytes(ep.InitialBytes), gzipTilde(v.GzipEstimated), formatBytes(ep.InitialGzipBytes))
			fmt.Fprintf(&sb, "  Async / Lazy Chunks:  %s (%d chunks)\n", formatBytes(ep.AsyncBytes), len(ep.ChunkIDs))
		}
		sb.WriteString("\n")
		if v.GzipEstimated {
			sb.WriteString("  gzip sizes are estimates (emitted files not found on disk)\n\n")
		}

		limit := 5
		if r.Top > 0 {
			limit = r.Top
		}
		top := v.TopPackages(limit)
		if len(top) > 0 {
			sb.WriteString("TOP CONTRIBUTING NPM PACKAGES\n")
			sb.WriteString("-------------------------------------------------------------\n")
			for i, pkg := range top {
				fmt.Fprintf(&sb, "%d. %-20s %10s  [~%s gzip] (%d modules)\n",
					i+1, pkg.Name, formatBytes(pkg.SizeBytes), formatBytes(pkg.GzipBytes), pkg.ModuleCount)
			}
			sb.WriteString("\n")
		}

	case *diff.BundleDiff:
		sb.WriteString("\n⚡ BUNDLERADAR COMPARISON & DIFF\n")
		sb.WriteString("-------------------------------------------------------------\n")
		s := v.Summary
		formatDelta := func(d int64) string {
			if d > 0 {
				return "+" + formatBytes(d)
			} else if d < 0 {
				return formatBytes(d)
			}
			return "0 B"
		}
		if s.HeadTotalBytes > 0 || s.BaseTotalBytes > 0 {
			fmt.Fprintf(&sb, "Initial JS:   %s → %s (%s)\n", formatBytes(s.BaseInitialBytes), formatBytes(s.HeadInitialBytes), formatDelta(s.InitialDeltaBytes))
			fmt.Fprintf(&sb, "Lazy JS:      %s → %s (%s)\n", formatBytes(s.BaseLazyBytes), formatBytes(s.HeadLazyBytes), formatDelta(s.LazyDeltaBytes))
			fmt.Fprintf(&sb, "Total JS:     %s → %s (%s)\n", formatBytes(s.BaseTotalBytes), formatBytes(s.HeadTotalBytes), formatDelta(s.TotalDeltaBytes))
		} else {
			for name, ep := range v.Entrypoints {
				sign := "+"
				if ep.InitialDelta < 0 {
					sign = ""
				}
				fmt.Fprintf(&sb, "Entrypoint %q Initial Delta: %s%s [~+%s gzip]\n",
					name, sign, formatBytes(ep.InitialDelta), formatBytes(ep.InitialGzipDelta))
			}
		}

		if len(v.Packages) > 0 {
			sb.WriteString("\nCHANGED PACKAGES\n")
			sb.WriteString("-------------------------------------------------------------\n")
			for _, pkg := range v.Packages {
				status := "🔄"
				if pkg.BaseBytes == 0 {
					status = "➕"
				} else if pkg.CurrBytes == 0 {
					status = "➖"
				}
				sign := "+"
				if pkg.DeltaBytes < 0 {
					sign = ""
				}
				chunkInfo := ""
				if len(pkg.ChunkNames) > 0 {
					chunkInfo = fmt.Sprintf(" (%s)", strings.Join(pkg.ChunkNames, ", "))
				}
				fmt.Fprintf(&sb, "%s %-22s %10s%s\n", status, pkg.Name, sign+formatBytes(pkg.DeltaBytes), chunkInfo)
				if pkg.ImportPath != "" {
					fmt.Fprintf(&sb, "   • Import path: %s\n", pkg.ImportPath)
				}
			}
		}

		if v.MicroDriftBytes > 0 {
			sb.WriteString("\n")
			fmt.Fprintf(&sb, "Micro-drift: %s across sub-threshold updates\n", formatBytes(v.MicroDriftBytes))
		}
		sb.WriteString("\n")

	case policy.EvaluationResult:
		if v.Passed {
			sb.WriteString("\n⚡ BUNDLERADAR GATE: ✅ PASSED\n")
			sb.WriteString("All bundle size limits and architectural policies are satisfied.\n\n")
		} else {
			sb.WriteString("\n⚡ BUNDLERADAR GATE: ❌ POLICY VIOLATION\n")
			sb.WriteString("-------------------------------------------------------------\n")
			for _, viol := range v.Violations {
				fmt.Fprintf(&sb, "[!] %s: %s\n", viol.Rule, viol.Message)
			}
			sb.WriteString("\n")
		}

	default:
		return fmt.Errorf("terminal reporter: unsupported data type %T", data)
	}

	_, err := io.WriteString(w, sb.String())
	return err
}

// gzipTilde returns "~" for estimated gzip sizes so the terminal output
// distinguishes measured from approximated values. Measured gzip (emitted
// files found on disk) is shown without the tilde.
func gzipTilde(estimated bool) string {
	if estimated {
		return "~"
	}
	return ""
}
