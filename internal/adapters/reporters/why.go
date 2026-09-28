package reporters

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
		res.Chains = append(res.Chains, WhyChain{
			Output:       outputName,
			Initial:      isInitial,
			Path:         cc.path,
			BytesInChunk: cc.bytes,
		})
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