package workspaces

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sonuKumar03/bundleradar/internal/core"
)

// Registry manages and executes workspace resolvers.
type Registry struct {
	resolvers []core.WorkspaceResolver
}

// DefaultRegistry returns a registry equipped with standard resolvers.
func DefaultRegistry() *Registry {
	r := &Registry{}
	// Order of evaluation:
	// 1. Nx (if nx.json exists)
	// 2. Monorepo (pnpm-workspace.yaml, package.json workspaces)
	r.Register(&NxResolver{})
	r.Register(&MonorepoResolver{})
	return r
}

// Register adds a resolver to the registry.
func (r *Registry) Register(resolver core.WorkspaceResolver) {
	r.resolvers = append(r.resolvers, resolver)
}

// Resolve discovers targets using the first matching resolver.
func (r *Registry) Resolve(ctx context.Context, root string) ([]core.Target, error) {
	for _, res := range r.resolvers {
		if res.Detect(root) {
			return res.Resolve(ctx, root)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "angular.json")); err == nil {
		return nil, fmt.Errorf("Angular CLI workspace detected, but automatic project discovery is unsupported; pass explicit targets with --app NAME=STATS[:DIST]")
	}
	return nil, fmt.Errorf("no supported workspace layout detected in %q; pass explicit targets with --app NAME=STATS[:DIST]", root)
}
