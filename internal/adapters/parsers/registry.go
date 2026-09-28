package parsers

import (
	"fmt"
	"os"

	"github.com/sonuKumar03/bundleradar/internal/core"
)

// Registry manages and selects bundle parsers.
type Registry struct {
	parsers []core.Parser
}

// DefaultRegistry creates a registry equipped with all standard built-in parsers.
func DefaultRegistry() *Registry {
	r := &Registry{}
	// Priority order:
	// 1. Angular (specialized esbuild with browser directory / index heuristics)
	// 2. Esbuild (standard metafile)
	// 3. Vite (manifest.json)
	// 4. Webpack (stats.json)
	r.Register(&AngularParser{})
	r.Register(&EsbuildParser{})
	r.Register(&ViteParser{})
	r.Register(&WebpackParser{})
	return r
}

// Register registers a new parser into the registry.
func (r *Registry) Register(p core.Parser) {
	r.parsers = append(r.parsers, p)
}

// Resolve identifies the appropriate parser for a target via explicit flag or content sniffing.
func (r *Registry) Resolve(target core.Target) (core.Parser, error) {
	if target.Bundler != "" {
		for _, p := range r.parsers {
			if p.Name() == target.Bundler {
				return p, nil
			}
		}
		return nil, core.UsageErrorf("unsupported bundler %q", target.Bundler)
	}

	if target.StatsPath == "" {
		return nil, fmt.Errorf("stats path is required to detect parser")
	}

	f, err := os.Open(target.StatsPath)
	if err != nil {
		return nil, fmt.Errorf("open stats file %q: %w", target.StatsPath, err)
	}
	defer f.Close()

	sniff := make([]byte, 65536)
	n, _ := f.Read(sniff)
	sample := sniff[:n]

	// If large file and outputs might be placed towards the end, append tail
	if fi, err := f.Stat(); err == nil && fi.Size() > int64(len(sample)) {
		tailSize := int64(32768)
		if fi.Size() > tailSize {
			tail := make([]byte, tailSize)
			if _, err := f.ReadAt(tail, fi.Size()-tailSize); err == nil {
				sample = append(sample, tail...)
			}
		}
	}

	for _, p := range r.parsers {
		if p.Detect(sample, target.DistPath) {
			return p, nil
		}
	}

	return nil, core.UsageErrorf("unable to detect bundler format for %q (pass --bundler to specify)", target.StatsPath)
}
