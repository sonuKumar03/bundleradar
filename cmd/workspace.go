package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/sonuKumar03/bundleradar/internal/adapters/workspaces"
	"github.com/sonuKumar03/bundleradar/internal/core"
	"github.com/sonuKumar03/bundleradar/pkg/bundleradar"
	"github.com/spf13/cobra"
)

func newWorkspaceCommand() *cobra.Command {
	var (
		root   string
		format string
		output string
		apps   []string
	)

	parent := &cobra.Command{
		Use:   "workspace",
		Short: "Discover and analyze applications across monorepos and multi-app workspaces",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all discovered application targets across the workspace",
		RunE: func(c *cobra.Command, args []string) error {
			if root == "" {
				root = "."
			}

			var targets []core.Target
			var err error

			if len(apps) > 0 {
				resolver := &workspaces.ExplicitResolver{Specs: apps}
				targets, err = resolver.Resolve(c.Context(), root)
			} else {
				targets, err = workspaces.DefaultRegistry().Resolve(c.Context(), root)
			}
			if err != nil {
				return err
			}

			w, cleanup, err := getOutputWriter(c, output)
			if err != nil {
				return err
			}
			defer func() { _ = cleanup() }()

			fmt.Fprintf(w, "Discovered %d application target(s) in %q:\n", len(targets), root)
			for i, t := range targets {
				fmt.Fprintf(w, "  %d. %s (stats: %s)\n", i+1, t.Name, t.StatsPath)
			}
			return nil
		},
	}

	scanCmd := &cobra.Command{
		Use:     "scan",
		Aliases: []string{"summary"},
		Short:   "Scan and summarize all discovered application targets in the workspace",
		RunE: func(c *cobra.Command, args []string) error {
			if root == "" {
				root = "."
			}

			var targets []core.Target
			var err error

			if len(apps) > 0 {
				resolver := &workspaces.ExplicitResolver{Specs: apps}
				targets, err = resolver.Resolve(c.Context(), root)
			} else {
				targets, err = workspaces.DefaultRegistry().Resolve(c.Context(), root)
			}
			if err != nil {
				return err
			}

			client := bundleradar.New()
			ctx := c.Context()

			w, cleanup, err := getOutputWriter(c, output)
			if err != nil {
				return err
			}
			defer func() { _ = cleanup() }()

			type targetScan struct {
				target core.Target
				bundle *core.Bundle
				err    error
			}

			scans := make([]targetScan, 0, len(targets))
			for _, t := range targets {
				b, err := client.Scan(ctx, bundleradar.ScanOptions{
					StatsPath: t.StatsPath,
					DistPath:  t.DistPath,
					Bundler:   t.Bundler,
				})
				scans = append(scans, targetScan{target: t, bundle: b, err: err})
			}

			if format == "json" {
				type TargetResult struct {
					Name         string `json:"name"`
					StatsPath    string `json:"statsPath"`
					DistPath     string `json:"distPath,omitempty"`
					InitialBytes int64  `json:"initialBytes"`
					AsyncBytes   int64  `json:"asyncBytes"`
					TotalBytes   int64  `json:"totalBytes"`
					ChunkCount   int    `json:"chunkCount"`
				}
				type WorkspaceResult struct {
					Targets           []TargetResult `json:"targets"`
					TotalInitialBytes int64          `json:"totalInitialBytes"`
					TotalAsyncBytes   int64          `json:"totalAsyncBytes"`
					TotalBytes        int64          `json:"totalBytes"`
				}

				res := WorkspaceResult{Targets: make([]TargetResult, 0, len(scans))}
				for _, s := range scans {
					if s.err != nil {
						continue
					}
					var chunkBytes int64
					for _, ch := range s.bundle.Chunks {
						chunkBytes += ch.SizeBytes
					}
					tr := TargetResult{
						Name:         s.target.Name,
						StatsPath:    s.target.StatsPath,
						DistPath:     s.target.DistPath,
						InitialBytes: s.bundle.TotalInitialBytes(),
						AsyncBytes:   s.bundle.TotalAsyncBytes(),
						TotalBytes:   chunkBytes,
						ChunkCount:   len(s.bundle.Chunks),
					}
					res.Targets = append(res.Targets, tr)
					res.TotalInitialBytes += tr.InitialBytes
					res.TotalAsyncBytes += tr.AsyncBytes
					res.TotalBytes += tr.TotalBytes
				}
				enc := json.NewEncoder(w)
				enc.SetIndent("", "  ")
				return enc.Encode(res)
			}

			if format == "markdown" {
				fmt.Fprintf(w, "## ⚡ Workspace Bundle Scan (%d targets)\n\n", len(targets))
				fmt.Fprintf(w, "| Application | Initial JS | Async JS | Chunks |\n")
				fmt.Fprintf(w, "| :--- | :--- | :--- | :---: |\n")
				for _, s := range scans {
					if s.err != nil {
						fmt.Fprintf(w, "| **`%s`** | *Error: %v* | - | - |\n", s.target.Name, s.err)
						continue
					}
					fmt.Fprintf(w, "| **`%s`** | `%s` | `%s` | %d |\n",
						s.target.Name, bundleradar.FormatBytes(s.bundle.TotalInitialBytes()), bundleradar.FormatBytes(s.bundle.TotalAsyncBytes()), len(s.bundle.Chunks))
				}
				fmt.Fprintf(w, "\n")
				return nil
			}

			fmt.Fprintf(w, "\n⚡ WORKSPACE BUNDLE SCAN (%d targets)\n", len(targets))
			fmt.Fprintf(w, "-------------------------------------------------------------\n")
			for _, s := range scans {
				if s.err != nil {
					fmt.Fprintf(w, "❌ %-20s Error: %v\n", s.target.Name, s.err)
					continue
				}
				fmt.Fprintf(w, "✓ %-20s Initial: %d bytes | Async: %d bytes (%d chunks)\n",
					s.target.Name, s.bundle.TotalInitialBytes(), s.bundle.TotalAsyncBytes(), len(s.bundle.Chunks))
			}
			fmt.Fprintf(w, "\n")
			return nil
		},
	}

	parent.PersistentFlags().StringVar(&root, "root", ".", "Root directory of the workspace")
	parent.PersistentFlags().StringSliceVar(&apps, "app", nil, "Explicit application target mapping name=stats[:dist]")
	parent.PersistentFlags().StringVarP(&format, "format", "f", "terminal", "Output format: terminal, markdown, json")
	parent.PersistentFlags().StringVarP(&output, "output", "o", "", "Write output to file path")

	parent.AddCommand(listCmd, scanCmd)
	return parent
}
