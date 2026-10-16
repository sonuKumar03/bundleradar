// Package bundleradar provides the public Go SDK for universal bundle inspection, diffing, and budget enforcement.
package bundleradar

import (
	"context"

	"github.com/sonuKumar03/bundleradar/internal/adapters/parsers"
	"github.com/sonuKumar03/bundleradar/internal/adapters/reporters"
	"github.com/sonuKumar03/bundleradar/internal/core"
	"github.com/sonuKumar03/bundleradar/internal/core/diff"
	"github.com/sonuKumar03/bundleradar/internal/core/policy"
)

// Re-export core types for client consumers
type Bundle = core.Bundle
type Entrypoint = core.Entrypoint
type Chunk = core.Chunk
type Module = core.Module
type Policy = policy.Policy
type EvaluationResult = policy.EvaluationResult
type BundleDiff = diff.BundleDiff

// ScanOptions configures bundle parsing.
type ScanOptions struct {
	StatsPath string
	DistPath  string
	Bundler   string // Optional: "esbuild", "angular", "vite", "webpack" (auto-detected if empty)
}

// Client is the primary interface for programmatic bundle operations.
type Client struct {
	parserRegistry *parsers.Registry
}

// New creates an initialized BundleRadar SDK client.
func New() *Client {
	return &Client{
		parserRegistry: parsers.DefaultRegistry(),
	}
}

// Scan parses and aggregates a bundle from build stats/metafile outputs.
func (c *Client) Scan(ctx context.Context, opts ScanOptions) (*core.Bundle, error) {
	target := core.Target{
		StatsPath: opts.StatsPath,
		DistPath:  opts.DistPath,
		Bundler:   opts.Bundler,
	}

	p, err := c.parserRegistry.Resolve(target)
	if err != nil {
		return nil, err
	}

	bundle, err := p.Parse(ctx, target)
	if err != nil {
		return nil, err
	}
	bundle.Warnings = bundle.CheckInvariants()
	return bundle, nil
}

// Diff compares a base bundle with a current bundle.
func (c *Client) Diff(base, current *core.Bundle, opts diff.Options) *diff.BundleDiff {
	return diff.Calculate(base, current, opts)
}

// Gate validates bundle metrics and deltas against a specified policy.
func (c *Client) Gate(bundle *core.Bundle, d *diff.BundleDiff, p policy.Policy) policy.EvaluationResult {
	return policy.Evaluate(bundle, d, p)
}

// Reporter retrieves an output reporter by format name.
func (c *Client) Reporter(format string) (core.Reporter, error) {
	return reporters.New(format)
}
