package cmd

import (
	"context"
	"fmt"
	"slices"

	"github.com/sonuKumar03/bundleradar/internal/core"
	"github.com/sonuKumar03/bundleradar/internal/core/diff"
	"github.com/sonuKumar03/bundleradar/pkg/bundleradar"
	"github.com/spf13/cobra"
)

func newGateCommand() *cobra.Command {
	var (
		dist                string
		bundler             string
		format              string
		output              string
		against             string
		buildCmd            string
		buildDir            string
		noBuild             bool
		maxInitial          string
		maxLazy             string
		maxTotal            string
		maxInitialDelta     string
		maxTotalDelta       string
		maxCSS              string
		maxAssets           string
		forbid              []string
		detectDuplicatePkgs bool
		entry               string
	)

	c := &cobra.Command{
		Use:   "gate [stats.json]",
		Short: "Validate bundle sizes, regressions, and architecture rules against policy budgets in CI",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			statsPath := ""
			if len(args) > 0 {
				statsPath = args[0]
			}
			if statsPath == "" {
				return &UsageError{Err: fmt.Errorf("stats path is required")}
			}

			client := bundleradar.New()
			ctx := context.Background()

			bundle, err := client.Scan(ctx, bundleradar.ScanOptions{
				StatsPath: statsPath,
				DistPath:  dist,
				Bundler:   bundler,
			})
			if err != nil {
				return fmt.Errorf("scan bundle: %w", err)
			}

			if entry != "" {
				bundle, err = scopeBundle(bundle, entry)
				if err != nil {
					return err
				}
			}

			var diffResult *bundleradar.BundleDiff
			if against != "" {
				baseBundle, cleanupBase, err := resolveBaselineBundle(ctx, client, against, statsPath, bundler, buildCmd, buildDir, noBuild, c.OutOrStdout(), format)
				if err != nil {
					return err
				}
				defer cleanupBase()
				if entry != "" {
					baseBundle, err = scopeBundle(baseBundle, entry)
					if err != nil {
						return err
					}
				}
				diffResult = client.Diff(baseBundle, bundle, diff.Options{})
			}

			cfg, _, err := bundleradar.FindAndLoadConfig("")
			if err != nil {
				return &UsageError{Err: err}
			}
			pol, err := cfg.ToPolicy()
			if err != nil {
				return &UsageError{Err: err}
			}
			if maxInitial != "" {
				val, err := bundleradar.ParseBytes(maxInitial)
				if err != nil {
					return &UsageError{Err: fmt.Errorf("invalid --max-initial: %w", err)}
				}
				pol.MaxInitial = &val
			}
			if maxTotal != "" {
				val, err := bundleradar.ParseBytes(maxTotal)
				if err != nil {
					return &UsageError{Err: fmt.Errorf("invalid --max-total: %w", err)}
				}
				pol.MaxTotal = &val
			}
			if maxLazy != "" {
				val, err := bundleradar.ParseBytes(maxLazy)
				if err != nil {
					return &UsageError{Err: fmt.Errorf("invalid --max-lazy: %w", err)}
				}
				pol.MaxLazy = &val
			}
			if maxInitialDelta != "" {
				val, err := bundleradar.ParseBytes(maxInitialDelta)
				if err != nil {
					return &UsageError{Err: fmt.Errorf("invalid --max-initial-delta: %w", err)}
				}
				pol.MaxInitialDelta = &val
			}
			if maxTotalDelta != "" {
				val, err := bundleradar.ParseBytes(maxTotalDelta)
				if err != nil {
					return &UsageError{Err: fmt.Errorf("invalid --max-total-delta: %w", err)}
				}
				pol.MaxTotalDelta = &val
			}
			if maxCSS != "" {
				val, err := bundleradar.ParseBytes(maxCSS)
				if err != nil {
					return &UsageError{Err: fmt.Errorf("invalid --max-css: %w", err)}
				}
				pol.MaxCSS = &val
			}
			if maxAssets != "" {
				val, err := bundleradar.ParseBytes(maxAssets)
				if err != nil {
					return &UsageError{Err: fmt.Errorf("invalid --max-assets: %w", err)}
				}
				pol.MaxAssets = &val
			}
			if c.Flags().Changed("forbid") {
				pol.ForbiddenPkgs = forbid
			}
			pol.DetectDuplicatePkgs = detectDuplicatePkgs
			if (pol.MaxInitialDelta != nil || pol.MaxTotalDelta != nil) && against == "" {
				return &UsageError{Err: fmt.Errorf("delta budgets require --against")}
			}

			evalRes := client.Gate(bundle, diffResult, pol)

			w, cleanup, err := getOutputWriter(c, output)
			if err != nil {
				return err
			}
			defer func() { _ = cleanup() }()

			rep, err := client.Reporter(format)
			if err != nil {
				return err
			}

			if err := rep.Render(ctx, w, evalRes); err != nil {
				return err
			}

			if !evalRes.Passed {
				return &PolicyViolationError{Err: fmt.Errorf("bundle policy violation")}
			}

			return nil
		},
	}

	c.Flags().StringVarP(&dist, "dist", "d", "", "Path to emitted browser dist with index.html")
	c.Flags().StringVar(&bundler, "bundler", "", "Override bundler auto-detection")
	c.Flags().StringVarP(&format, "format", "f", "terminal", "Output format: terminal, markdown, github-pr, json")
	c.Flags().StringVarP(&output, "output", "o", "", "Write output to file path")
	c.Flags().StringVar(&against, "against", "", "Baseline stats/metafile or git ref to enforce delta limits")
	c.Flags().StringVar(&buildCmd, "build-cmd", "npm run build", "Build command to execute inside temporary worktree")
	c.Flags().StringVar(&buildDir, "build-dir", "", "Project directory (relative to worktree root) containing package.json for monorepo baselines")
	c.Flags().BoolVar(&noBuild, "no-build", false, "Skip building inside temporary worktree")
	c.Flags().StringVar(&maxInitial, "max-initial", "", "Maximum allowed initial bundle size (e.g. 250KB, 1MB)")
	c.Flags().StringVar(&maxLazy, "max-lazy", "", "Maximum allowed lazy bundle size (e.g. 500KB)")
	c.Flags().StringVar(&maxTotal, "max-total", "", "Maximum allowed total bundle size (e.g. 1.5MB)")
	c.Flags().StringVar(&maxInitialDelta, "max-initial-delta", "", "Maximum allowed increase vs baseline")
	c.Flags().StringVar(&maxTotalDelta, "max-total-delta", "", "Maximum allowed total JS increase vs baseline")
	c.Flags().StringVar(&maxCSS, "max-css", "", "Maximum allowed total CSS size (e.g. 100KB, 1MB)")
	c.Flags().StringVar(&maxAssets, "max-assets", "", "Maximum allowed total asset size for images, fonts and media (e.g. 2MB)")
	c.Flags().StringSliceVar(&forbid, "forbid", nil, "Forbidden package names (e.g. moment,lodash)")
	c.Flags().BoolVar(&detectDuplicatePkgs, "detect-duplicate-pkgs", true, "Fail if multiple versions of the same package are bundled")
	c.Flags().StringVarP(&entry, "entry", "e", "", "Scope budget checks to a specific entrypoint")

	return c
}

func scopeBundle(bundle *core.Bundle, query string) (*core.Bundle, error) {
	ep, ok := bundle.ResolveEntrypoint(query)
	if !ok {
		return nil, &UsageError{Err: fmt.Errorf("entrypoint %q not found in bundle", query)}
	}
	scoped := core.NewBundle(bundle.Metadata)
	scoped.AddEntrypoint(ep.Name, *ep)
	for _, chunk := range bundle.Chunks {
		scoped.AddChunk(chunk)
	}
	for _, mod := range bundle.Modules {
		for _, id := range mod.ChunkIDs {
			if slices.Contains(ep.ChunkIDs, id) || len(bundle.Entrypoints) == 1 {
				scoped.AddModule(mod)
				break
			}
		}
	}
	return scoped, nil
}
