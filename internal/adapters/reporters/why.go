package reporters

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/sonuKumar03/bundleradar/internal/core"
)

// WhyChain describes one emitted output carrying bytes for the traced target.
type WhyChain struct {
	Output       string   `json:"output"`
	Initial      bool     `json:"initial"`
	Path         []string `json:"path"`
	BytesInChunk int64    `json:"bytesInChunk"`
}

// WhyResult is the structured outcome of tracing a package or module.
type WhyResult struct {
	Target       string     `json:"target"`
	PackageName  string     `json:"packageName"`
	Found        bool       `json:"found"`
	InitialBytes int64      `json:"initialBytes"`
	LazyBytes    int64      `json:"lazyBytes"`
	TotalBytes   int64      `json:"totalBytes"`
	Chains       []WhyChain `json:"chains"`
	// RetainedInitialBytes is the initial JS that would drop out of the
	// startup bundle if no other module statically imported the target.
	// It is omitted when the bundler's stats carry no import edges.
	RetainedInitialBytes int64         `json:"retainedInitialBytes,omitempty"`
	Importers            []WhyImporter `json:"importers,omitempty"`
}

// WhyImporter is a module that statically imports the target.
type WhyImporter struct {
	Module     string `json:"module"`
	Package    string `json:"package,omitempty"`
	FirstParty bool   `json:"firstParty"`
	// SavingBytes is the initial JS saved by removing only this importer's
	// import of the target. It is 0 when other importers keep it in the bundle.
	SavingBytes int64 `json:"savingBytes"`
}

// WhyReporter renders an import trace for a target. It implements core.Reporter
// so the same trace logic serves the CLI, server, and MCP consumers.
type WhyReporter struct {
	Target     string
	FormatName string // "terminal" (default) or "json"
}

// NewWhy creates a WhyReporter bound to a trace target.
func NewWhy(target, format string) *WhyReporter {
	return &WhyReporter{Target: target, FormatName: strings.ToLower(strings.TrimSpace(format))}
}

func (r *WhyReporter) Format() string {
	if r.FormatName == "json" {
		return "json"
	}
	return "terminal"
}

// Render computes the import trace of r.Target within bundle and writes the
// result in the configured format.
func (r *WhyReporter) Render(ctx context.Context, w io.Writer, data any) error {
	bundle, ok := data.(*core.Bundle)
	if !ok {
		return fmt.Errorf("why reporter: expected *core.Bundle, got %T", data)
	}

	chunkMap := make(map[string]core.Chunk, len(bundle.Chunks))
	for _, ch := range bundle.Chunks {
		chunkMap[ch.ID] = ch
	}

	normTarget := strings.TrimSpace(r.Target)
	res := WhyResult{
		Target:      normTarget,
		PackageName: normTarget,
		Chains:      make([]WhyChain, 0),
	}

	type chunkContrib struct {
		bytes int64
		path  []string
	}
	contribs := make(map[string]*chunkContrib)
	targetIDs := make(map[string]bool)

	for _, m := range bundle.Modules {
		if err := ctx.Err(); err != nil {
			return err
		}
		matched := m.Package == normTarget || strings.EqualFold(m.Package, normTarget)
		if !matched && m.Package == "" {
			matched = strings.Contains(m.ID, normTarget)
		}
		if !matched {
			continue
		}

		res.Found = true
		targetIDs[m.ID] = true
		if m.Package != "" {
			res.PackageName = m.Package
		}

		for _, cid := range m.ChunkIDs {
			cc, ok := contribs[cid]
			if !ok {
				cc = &chunkContrib{}
				contribs[cid] = cc
			}
			if size, ok := m.ChunkBytes[cid]; ok {
				cc.bytes += size
			} else {
				cc.bytes += m.SizeBytes
			}
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
		res.Chains = append(res.Chains, WhyChain{
			Output:       outputName,
			Initial:      isInitial,
			Path:         cc.path,
			BytesInChunk: cc.bytes,
		})
	}

	sort.SliceStable(res.Chains, func(i, j int) bool {
		a, b := res.Chains[i], res.Chains[j]
		if a.Initial != b.Initial {
			return a.Initial
		}
		if a.BytesInChunk != b.BytesInChunk {
			return a.BytesInChunk > b.BytesInChunk
		}
		return a.Output < b.Output
	})
	if res.Found && bundle.Graph != nil {
		res.RetainedInitialBytes, res.Importers = traceImporters(bundle, targetIDs)
	}

	if r.FormatName == "json" {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}

	if !res.Found {
		fmt.Fprintf(w, "\n❌ Package %q not found in bundle\n\n", r.Target)
		return nil
	}

	fmt.Fprintf(w, "\n⚡ BUNDLERADAR IMPORT TRACE: %s\n", res.PackageName)
	fmt.Fprintf(w, "-------------------------------------------------------------\n")
	fmt.Fprintf(w, "Total Size:   %s (Initial: %s, Lazy: %s)\n",
		formatBytes(res.TotalBytes), formatBytes(res.InitialBytes), formatBytes(res.LazyBytes))
	if res.RetainedInitialBytes > 0 || len(res.Importers) > 0 {
		fmt.Fprintf(w, "Retained:     %s of initial JS depends only on this package's imports\n", formatBytes(res.RetainedInitialBytes))
		if len(res.Importers) > 0 {
			fmt.Fprintf(w, "Static importers (removing one import saves its SavingBytes):\n")
			for i, imp := range res.Importers {
				if i == 15 {
					fmt.Fprintf(w, "  … and %d more\n", len(res.Importers)-i)
					break
				}
				kind := "pkg"
				if imp.FirstParty {
					kind = "app"
				}
				fmt.Fprintf(w, "  • [%s] %s (saves %s)\n", kind, imp.Module, formatBytes(imp.SavingBytes))
			}
		}
	}
	fmt.Fprintf(w, "Emitted into %d chunk(s):\n\n", len(res.Chains))
	for _, c := range res.Chains {
		kind := "lazy"
		if c.Initial {
			kind = "initial"
		}
		fmt.Fprintf(w, "  • Output: %s [%s] (%s)\n", c.Output, kind, formatBytes(c.BytesInChunk))
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

// traceImporters computes the retained initial bytes of the target modules and
// each static importer's individual saving using the bundle's import graph.
func traceImporters(bundle *core.Bundle, targets map[string]bool) (int64, []WhyImporter) {
	g := bundle.Graph
	modByID := make(map[string]core.Module, len(bundle.Modules))
	for _, m := range bundle.Modules {
		modByID[m.ID] = m
	}
	initialChunks := bundle.InitialChunkSet()

	// esbuild lists unused imports in the metafile, so a static edge can lead
	// to a module that only ships in a lazy chunk. Never traverse into those.
	lazyOnly := make(map[string]bool)
	for _, m := range bundle.Modules {
		if len(m.ChunkIDs) > 0 && core.ModuleInitialBytes(m, initialChunks) == 0 {
			lazyOnly[m.ID] = true
		}
	}
	reach := func(skip func(from, to string) bool) map[string]bool {
		return g.StaticReach(func(from, to string) bool {
			return lazyOnly[to] || (skip != nil && skip(from, to))
		})
	}

	before := reach(nil)
	savedBy := func(skip func(from, to string) bool) int64 {
		after := reach(skip)
		var saved int64
		for id := range before {
			if !after[id] {
				saved += core.ModuleInitialBytes(modByID[id], initialChunks)
			}
		}
		return saved
	}

	retained := savedBy(func(from, to string) bool { return targets[to] && !targets[from] })

	froms := make([]string, 0)
	for from, edges := range g.Edges {
		if targets[from] || !before[from] {
			continue
		}
		for _, e := range edges {
			if !e.Dynamic && targets[e.To] {
				froms = append(froms, from)
				break
			}
		}
	}
	importers := make([]WhyImporter, 0, len(froms))
	for _, from := range froms {
		m := modByID[from]
		importers = append(importers, WhyImporter{
			Module:      from,
			Package:     m.Package,
			FirstParty:  m.Package == "",
			SavingBytes: savedBy(func(f, to string) bool { return f == from && targets[to] }),
		})
	}
	sort.Slice(importers, func(i, j int) bool {
		a, b := importers[i], importers[j]
		if a.FirstParty != b.FirstParty {
			return a.FirstParty
		}
		if a.SavingBytes != b.SavingBytes {
			return a.SavingBytes > b.SavingBytes
		}
		return a.Module < b.Module
	})
	return retained, importers
}
