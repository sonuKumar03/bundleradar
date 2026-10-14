package parsers

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/sonuKumar03/bundleradar/internal/core"
)

type WebpackStats struct {
	AssetsByChunkName map[string]any   `json:"assetsByChunkName"`
	Chunks            []WebpackChunk   `json:"chunks"`
	Assets            []WebpackAsset   `json:"assets"`
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
	Name       string              `json:"name"`
	Size       int64               `json:"size"`
	IssuerName string              `json:"issuerName"`
	IssuerPath []WebpackIssuerStep `json:"issuerPath"`
}

type WebpackIssuerStep struct {
	Name string `json:"name"`
}

type WebpackAsset struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

type WebpackParser struct{}

func (p *WebpackParser) Name() string {
	return "webpack"
}

func (p *WebpackParser) Detect(sample []byte, distDir string) bool {
	return hasJSONKeys(sample, "assetsByChunkName") ||
		(hasJSONKeys(sample, "chunks") && hasJSONKeys(sample, "modules"))
}

func (p *WebpackParser) Parse(ctx context.Context, target core.Target) (*core.Bundle, error) {
	var stats WebpackStats
	if err := decodeStats(ctx, target.StatsPath, &stats); err != nil {
		return nil, fmt.Errorf("parse webpack stats JSON: %w", err)
	}

	distDir := resolveDistDir(target.StatsPath, target.DistPath)

	bundle := core.NewBundle(core.Metadata{
		Bundler: "webpack",
	})

	seenModules := make(map[string]*core.Module)

	for _, wChunk := range stats.Chunks {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		chunkName := chunkDisplayName(wChunk)
		fileName := chunkName
		if len(wChunk.Files) > 0 {
			fileName = wChunk.Files[0]
		}

		loadType := core.LoadTypeAsync
		if wChunk.Initial || wChunk.Entry {
			loadType = core.LoadTypeInitial
		}

		gzBytes, estimated := gzipFor(distDir, fileName, wChunk.Size)
		chunk := core.Chunk{
			ID:            chunkName,
			Name:          fileName,
			Path:          fileName,
			SizeBytes:     wChunk.Size,
			GzipBytes:     gzBytes,
			GzipEstimated: estimated,
			Type:          loadType,
			Entry:         chunkName,
			ModuleIDs:     make([]string, 0, len(wChunk.Modules)),
		}
		if estimated {
			bundle.GzipEstimated = true
		}

		for _, m := range wChunk.Modules {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if m.Name == "" {
				continue
			}
			chunk.ModuleIDs = append(chunk.ModuleIDs, m.Name)
			pkgName := extractPackageName(m.Name)

			if existing, ok := seenModules[m.Name]; ok {
				existing.ChunkIDs = append(existing.ChunkIDs, chunkName)
				existing.SizeBytes += m.Size
				existing.GzipBytes += estimateGzip(m.Size, m.Name)
			} else {
				mod := &core.Module{
					ID:           m.Name,
					Package:      pkgName,
					SizeBytes:    m.Size,
					GzipBytes:    estimateGzip(m.Size, m.Name),
					IsAppCode:    pkgName == "",
					ChunkIDs:     []string{chunkName},
					IngressPaths: buildWebpackIngressPath(m),
				}
				seenModules[m.Name] = mod
			}
		}

		bundle.AddChunk(chunk)

		if wChunk.Initial || wChunk.Entry {
			bundle.AddEntrypoint(chunkName, core.Entrypoint{
				Name:             chunkName,
				InitialBytes:     wChunk.Size,
				InitialGzipBytes: gzBytes,
				ChunkIDs:         []string{chunkName},
			})
		}
	}

	// Add deduplicated modules in sorted order for determinism
	modIDs := make([]string, 0, len(seenModules))
	for id := range seenModules {
		modIDs = append(modIDs, id)
	}
	slices.Sort(modIDs)
	for _, id := range modIDs {
		bundle.AddModule(*seenModules[id])
	}

	// Register top-level assets
	for _, a := range stats.Assets {
		ext := strings.ToLower(filepath.Ext(a.Name))
		if ext == ".js" || ext == ".mjs" {
			continue
		}
		gzBytes, estimated := gzipFor(distDir, a.Name, a.Size)
		bundle.AddAsset(core.Asset{
			Path:          a.Name,
			SizeBytes:     a.Size,
			GzipBytes:     gzBytes,
			GzipEstimated: estimated,
			MimeType:      inferMimeType(ext),
		})
		if estimated {
			bundle.GzipEstimated = true
		}
	}

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

// buildWebpackIngressPath constructs an ingress chain from webpack's issuerPath.
func buildWebpackIngressPath(m WebpackModule) []string {
	if len(m.IssuerPath) > 0 {
		path := make([]string, 0, len(m.IssuerPath)+1)
		for _, step := range m.IssuerPath {
			if step.Name != "" {
				path = append(path, step.Name)
			}
		}
		path = append(path, m.Name)
		return path
	}
	if m.IssuerName != "" {
		return []string{m.IssuerName, m.Name}
	}
	return nil
}

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
