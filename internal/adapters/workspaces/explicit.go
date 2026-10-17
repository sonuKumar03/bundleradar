package workspaces

import (
	"context"
	"strings"

	"github.com/sonuKumar03/bundleradar/internal/core"
)

// ExplicitResolver parses direct user-provided target specifications.
type ExplicitResolver struct {
	Specs []string // Specs in format name=stats[:dist]
}

func (r *ExplicitResolver) Name() string {
	return "explicit"
}

func (r *ExplicitResolver) Detect(root string) bool {
	return len(r.Specs) > 0
}

func (r *ExplicitResolver) Resolve(ctx context.Context, root string) ([]core.Target, error) {
	var targets []core.Target
	for _, spec := range r.Specs {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			continue
		}
		eqIdx := strings.Index(spec, "=")
		if eqIdx <= 0 {
			return nil, core.UsageErrorf("invalid target spec %q: expected format name=stats_path[:dist_path]", spec)
		}
		name := strings.TrimSpace(spec[:eqIdx])
		rest := strings.TrimSpace(spec[eqIdx+1:])

		var statsPath, distPath string
		colonIdx := strings.Index(rest, ":")
		if colonIdx > 0 {
			statsPath = rest[:colonIdx]
			distPath = rest[colonIdx+1:]
		} else {
			statsPath = rest
		}

		targets = append(targets, core.Target{
			Name:      name,
			StatsPath: statsPath,
			DistPath:  distPath,
		})
	}
	return targets, nil
}
