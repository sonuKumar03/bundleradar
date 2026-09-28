package parsers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sonuKumar03/bundleradar/internal/core"
)

type WebpackStats struct {
	AssetsByChunkName map[string]any  `json:"assetsByChunkName"`
	Chunks            []WebpackChunk  `json:"chunks"`
}

type WebpackChunk struct {
	ID      any             `json:"id"`
	Names   []string        `json:"names"`
	Files   []string        `json:"files"`
	Size    int64           `json:"size"`
	Initial bool            `json:"initial"`
	Entry   bool            `json:"entry"`
	Modules []WebpackModule `json:"modules"`
}

type WebpackModule struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

type WebpackParser struct{}

func (p *WebpackParser) Name() string {
	return "webpack"
}

func (p *WebpackParser) Detect(sample []byte, distDir string) bool {
	return bytes.Contains(sample, []byte(`"assetsByChunkName"`)) ||
		(bytes.Contains(sample, []byte(`"chunks"`)) && bytes.Contains(sample, []byte(`"modules"`)))
}

func (p *WebpackParser) Parse(ctx context.Context, target core.Target) (*core.Bundle, error) {
	data, err := os.ReadFile(target.StatsPath)
	if err != nil {
		return nil, fmt.Errorf("read webpack stats %q: %w", target.StatsPath, err)
	}

	var stats WebpackStats
	if err := json.Unmarshal(data, &stats); err != nil {
		return nil, fmt.Errorf("parse webpack stats JSON: %w", err)
	}

	bundle := core.NewBundle(core.Metadata{
		Bundler: "webpack",
	})

	for _, wChunk := range stats.Chunks {
		chunkName := chunkDisplayName(wChunk)
		fileName := chunkName
		if len(wChunk.Files) > 0 {
			fileName = wChunk.Files[0]
		}

		loadType := core.LoadTypeAsync
		if wChunk.Initial || wChunk.Entry {
			loadType = core.LoadTypeInitial
		}

		chunk := core.Chunk{
			ID:        chunkName,
			Name:      fileName,
			Path:      fileName,
			SizeBytes: wChunk.Size,
			GzipBytes: estimateGzip(wChunk.Size),
			Type:      loadType,
			Entry:     chunkName,
			ModuleIDs: make([]string, 0, len(wChunk.Modules)),
		}

		for _, m := range wChunk.Modules {
			// Real webpack 5 stats nest aggregate rollups (e.g. "runtime
			// modules", "dependent modules") alongside real modules; those
			// carry no name and must not become phantom attribution rows.
			if m.Name == "" {
				continue
			}
			chunk.ModuleIDs = append(chunk.ModuleIDs, m.Name)
			pkgName := extractPackageName(m.Name)

			bundle.AddModule(core.Module{
				ID:        m.Name,
				Package:   pkgName,
				SizeBytes: m.Size,
				GzipBytes: estimateGzip(m.Size),
				IsAppCode: pkgName == "",
				ChunkIDs:  []string{chunkName},
			})
		}

		bundle.AddChunk(chunk)

		if wChunk.Initial || wChunk.Entry {
			bundle.AddEntrypoint(chunkName, core.Entrypoint{
				Name:             chunkName,
				InitialBytes:     wChunk.Size,
				InitialGzipBytes: estimateGzip(wChunk.Size),
				ChunkIDs:         []string{chunkName},
			})
		}
	}

	// Register entrypoints. Entrypoint async bytes are only attributed when
	// there is exactly one entrypoint, where every non-initial chunk is
	// unambiguously lazy code for it; multi-entry stats carry no reliable
	// per-entry chunk mapping, so they report 0 rather than a guess.
	if len(bundle.Entrypoints) == 1 {
		var asyncBytes int64
		for _, ch := range bundle.Chunks {
			if ch.Type == core.LoadTypeAsync {
				asyncBytes += ch.SizeBytes
			}
		}
		for name, ep := range bundle.Entrypoints {
			ep.AsyncBytes = asyncBytes
			bundle.Entrypoints[name] = ep
		}
	}

	return bundle, nil
}

// chunkDisplayName derives a stable chunk identifier from real webpack 5 stats.
// Modern stats may omit both id and names (e.g. optimization-generated async
// chunks), so fall back to the emitted file name without extension.
func chunkDisplayName(c WebpackChunk) string {
	if len(c.Names) > 0 {
		return c.Names[0]
	}
	if c.ID != nil {
		return fmt.Sprintf("%v", c.ID)
	}
	if len(c.Files) > 0 {
		if ext := filepath.Ext(c.Files[0]); ext != "" {
			return strings.TrimSuffix(c.Files[0], ext)
		}
		return c.Files[0]
	}
	return "unknown"
}
