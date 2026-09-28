package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/sonuKumar03/bundleradar/internal/adapters/reporters"
	"github.com/sonuKumar03/bundleradar/internal/core"
	"github.com/sonuKumar03/bundleradar/pkg/bundleradar"
	"github.com/spf13/cobra"
)

func newScanCommand() *cobra.Command {
	var (
		dist    string
		bundler string
		format  string
		output  string
		entry   string
		why     string
		top     int
		uiMode  bool
	)

	c := &cobra.Command{
		Use:   "scan [stats.json]",
		Short: "Inspect bundle sizes, breakdown, and package dependencies across entrypoints",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			statsPath := ""
			if len(args) > 0 {
				statsPath = args[0]
			}
			if statsPath == "" {
				return &UsageError{Err: fmt.Errorf("stats path is required")}
			}

			if uiMode {
				return runUIServerWithContext(c.Context(), "127.0.0.1", 4200, statsPath, true, true)
			}

			client := bundleradar.New()
			ctx := context.Background()

			bundle, err := client.Scan(ctx, bundleradar.ScanOptions{
				StatsPath: statsPath,
				DistPath:  dist,
				Bundler:   bundler,
			})
			if err != nil {
				return err
			}

			if entry != "" {
				ep, ok := bundle.ResolveEntrypoint(entry)
				if !ok {
					return &UsageError{Err: fmt.Errorf("entrypoint %q not found in bundle", entry)}
				}
				bundle.Entrypoints = map[string]core.Entrypoint{
					ep.Name: *ep,
				}
			}

			w, cleanup, err := getOutputWriter(c, output)
			if err != nil {
				return err
			}
			defer func() { _ = cleanup() }()

			if why != "" {
				return renderWhy(w, bundle, why, format)
			}

			rep, err := client.Reporter(format)
			if err != nil {
				return err
			}
			if tr, ok := rep.(*reporters.TerminalReporter); ok && top > 0 {
				tr.Top = top
			}

			return rep.Render(ctx, w, bundle)
		},
	}

	c.Flags().StringVarP(&dist, "dist", "d", "", "Path to emitted browser dist with index.html")
	c.Flags().StringVar(&bundler, "bundler", "", "Override bundler auto-detection (esbuild, angular, vite, webpack)")
	c.Flags().StringVarP(&format, "format", "f", "terminal", "Output format: terminal, markdown, json")
	c.Flags().StringVarP(&output, "output", "o", "", "Write output to file path")
	c.Flags().StringVarP(&entry, "entry", "e", "", "Scope scan to a specific entrypoint")
	c.Flags().StringVar(&why, "why", "", "Trace import path root for a specific package")
	c.Flags().IntVar(&top, "top", 10, "Number of top packages to list")
	c.Flags().BoolVar(&uiMode, "ui", false, "Launch interactive BundleRadar Studio web UI after scan")

	return c
}

type whyChain struct {
	Output       string   `json:"output"`
	Initial      bool     `json:"initial"`
	Path         []string `json:"path"`
	BytesInChunk int64    `json:"bytesInChunk"`
}

type whyResult struct {
	Target       string     `json:"target"`
	PackageName  string     `json:"packageName"`
	Found        bool       `json:"found"`
	InitialBytes int64      `json:"initialBytes"`
	LazyBytes    int64      `json:"lazyBytes"`
	TotalBytes   int64      `json:"totalBytes"`
	Chains       []whyChain `json:"chains"`
}

func renderWhy(w io.Writer, bundle *core.Bundle, target string, format string) error {
	chunkMap := make(map[string]core.Chunk, len(bundle.Chunks))
	for _, ch := range bundle.Chunks {
		chunkMap[ch.ID] = ch
	}

	normTarget := strings.TrimSpace(target)
	res := whyResult{
		Target:      normTarget,
		PackageName: normTarget,
		Chains:      make([]whyChain, 0),
	}

	type chunkContrib struct {
		bytes int64
		path  []string
	}
	contribs := make(map[string]*chunkContrib)

	for _, m := range bundle.Modules {
		matched := m.Package == normTarget || strings.EqualFold(m.Package, normTarget)
		if !matched && m.Package == "" {
			matched = strings.Contains(m.ID, normTarget)
		}
		if !matched {
			continue
		}

		res.Found = true
		if m.Package != "" {
			res.PackageName = m.Package
		}

		for _, cid := range m.ChunkIDs {
			cc, ok := contribs[cid]
			if !ok {
				cc = &chunkContrib{}
				contribs[cid] = cc
			}
			cc.bytes += m.SizeBytes
			if len(m.IngressPaths) > 0 && (len(cc.path) == 0 || len(m.IngressPaths) < len(cc.path)) {
				cc.path = m.IngressPaths
			}
		}
	}

	for cid, cc := range contribs {
		ch := chunkMap[cid]
		isInitial := ch.Type == core.LoadTypeInitial
		if isInitial {
			res.InitialBytes += cc.bytes
		} else {
			res.LazyBytes += cc.bytes
		}
		res.TotalBytes += cc.bytes

		outputName := ch.Name
		if outputName == "" {
			outputName = cid
		}
		res.Chains = append(res.Chains, whyChain{
			Output:       outputName,
			Initial:      isInitial,
			Path:         cc.path,
			BytesInChunk: cc.bytes,
		})
	}

	if format == "json" {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}

	if !res.Found {
		fmt.Fprintf(w, "\n❌ Package %q not found in bundle\n\n", target)
		return nil
	}

	fmt.Fprintf(w, "\n⚡ BUNDLERADAR IMPORT TRACE: %s\n", res.PackageName)
	fmt.Fprintf(w, "-------------------------------------------------------------\n")
	fmt.Fprintf(w, "Total Size:   %s (Initial: %s, Lazy: %s)\n",
		bundleradar.FormatBytes(res.TotalBytes), bundleradar.FormatBytes(res.InitialBytes), bundleradar.FormatBytes(res.LazyBytes))
	fmt.Fprintf(w, "Emitted into %d chunk(s):\n\n", len(res.Chains))
	for _, c := range res.Chains {
		kind := "lazy"
		if c.Initial {
			kind = "initial"
		}
		fmt.Fprintf(w, "  • Output: %s [%s] (%s)\n", c.Output, kind, bundleradar.FormatBytes(c.BytesInChunk))
		if len(c.Path) > 0 {
			fmt.Fprintf(w, "    Ingress Trail:\n")
			for i, step := range c.Path {
				prefix := "      ↳ "
				if i == 0 {
					prefix = "      ● "
				}
				fmt.Fprintf(w, "%s%s\n", prefix, step)
			}
		}
		fmt.Fprintf(w, "\n")
	}
	return nil
}
