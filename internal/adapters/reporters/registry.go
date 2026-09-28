package reporters

import (
	"fmt"
	"strings"

	"github.com/sonuKumar03/bundleradar/internal/core"
)

// New instantiates a reporter matching the requested format name.
func New(format string) (core.Reporter, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "markdown", "md":
		return &MarkdownReporter{}, nil
	case "github-pr", "pr":
		return &GitHubPRReporter{}, nil
	case "json":
		return &JSONReporter{}, nil
	case "terminal", "text", "":
		return &TerminalReporter{}, nil
	default:
		return nil, core.UsageErrorf("unsupported reporter format %q (use terminal, markdown, github-pr, or json)", format)
	}
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
