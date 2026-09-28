package parsers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sonuKumar03/bundleradar/internal/core"
)

type ViteChunk struct {
	File           string   `json:"file"`
	Src            string   `json:"src"`
	IsEntry        bool     `json:"isEntry"`
	IsDynamicEntry bool     `json:"isDynamicEntry"`
	Imports        []string `json:"imports"`
	DynamicImports []string `json:"dynamicImports"`
	Css            []string `json:"css"`
	Assets         []string `json:"assets"`
}

type ViteParser struct{}

func (p *ViteParser) Name() string {
	return "vite"
}

func (p *ViteParser) Detect(sample []byte, distDir string) bool {
	// Vite manifests declare per-entry objects carrying "file" and entry flags.
	return hasJSONKeys(sample, "file") &&
		(hasJSONKeys(sample, "isEntry") || hasJSONKeys(sample, "isDynamicEntry"))
}

func (p *ViteParser) Parse(ctx context.Context, target core.Target) (*core.Bundle, error) {
	data, err := os.ReadFile(target.StatsPath)
	if err != nil {
		return nil, fmt.Errorf("read vite manifest %q: %w", target.StatsPath, err)
	}

	var manifest map[string]ViteChunk
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse vite manifest JSON: %w", err)
	}

	bundle := core.NewBundle(core.Metadata{
		Bundler: "vite",
	})

	distDir := resolveDistDir(target.StatsPath, target.DistPath)

	for srcKey, vChunk := range manifest {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		sizeBytes := fileSize(distDir, vChunk.File)

		loadType := core.LoadTypeAsync
		if vChunk.IsEntry {
			loadType = core.LoadTypeInitial
		}

		chunkID := filepath.Base(vChunk.File)
		gzBytes, estimated := gzipFor(distDir, vChunk.File, sizeBytes)
		chunk := core.Chunk{
			ID:            chunkID,
			Name:          chunkID,
			Path:          vChunk.File,
			SizeBytes:     sizeBytes,
			GzipBytes:     gzBytes,
			GzipEstimated: estimated,
			Type:          loadType,
			Entry:         srcKey,
			ModuleIDs:     []string{srcKey},
		}
		if estimated {
			bundle.GzipEstimated = true
		}

		pkgName := extractPackageName(srcKey)
		bundle.AddModule(core.Module{
			ID:        srcKey,
			Package:   pkgName,
			SizeBytes: sizeBytes,
			GzipBytes: estimateGzip(sizeBytes, vChunk.File),
			IsAppCode: pkgName == "",
			ChunkIDs:  []string{chunkID},
		})

		bundle.AddChunk(chunk)

		if vChunk.IsEntry {
			bundle.AddEntrypoint(srcKey, core.Entrypoint{
				Name:             srcKey,
				InitialBytes:     sizeBytes,
				InitialGzipBytes: gzBytes,
				ChunkIDs:         []string{chunkID},
			})
		}

		// Add emitted CSS as assets
		for _, cssFile := range vChunk.Css {
			cssSize := fileSize(distDir, cssFile)
			cssGz, cssEstimated := gzipFor(distDir, cssFile, cssSize)
			bundle.AddAsset(core.Asset{
				Path:          cssFile,
				SizeBytes:     cssSize,
				GzipBytes:     cssGz,
				GzipEstimated: cssEstimated,
				MimeType:      "text/css",
			})
			if cssEstimated {
				bundle.GzipEstimated = true
			}
		}
	}

	// Attribute lazy-chunk bytes to entrypoints by walking each entry's
	// dynamicImports chain through the manifest. Without this, entrypoint
	// asyncBytes stays 0 even for apps with real dynamic imports.
	chunkSize := make(map[string]int64, len(manifest))
	for _, vChunk := range manifest {
		chunkSize[vChunk.File] = fileSize(distDir, vChunk.File)
	}
	for srcKey, vChunk := range manifest {
		if !vChunk.IsEntry {
			continue
		}
		var asyncBytes int64
		visited := make(map[string]bool)
		queue := append([]string{}, vChunk.DynamicImports...)
		for len(queue) > 0 {
			curr := queue[0]
			queue = queue[1:]
			if visited[curr] {
				continue
			}
			visited[curr] = true
			next, ok := manifest[curr]
			if !ok {
				continue
			}
			asyncBytes += chunkSize[next.File]
			queue = append(queue, next.DynamicImports...)
		}
		if asyncBytes > 0 {
			ep := bundle.Entrypoints[srcKey]
			ep.AsyncBytes = asyncBytes
			bundle.Entrypoints[srcKey] = ep
		}
	}

	return bundle, nil
}

func fileSize(distDir, relPath string) int64 {
	if fi, err := os.Stat(filepath.Join(distDir, relPath)); err == nil {
		return fi.Size()
	}
	return 0
}
