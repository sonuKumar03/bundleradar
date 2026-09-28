package parsers

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/sonuKumar03/bundleradar/internal/core"
)

type EsbuildMetafile struct {
	Inputs  map[string]EsbuildInput  `json:"inputs"`
	Outputs map[string]EsbuildOutput `json:"outputs"`
}

type EsbuildInput struct {
	Bytes   int64 `json:"bytes"`
	Imports []struct {
		Path string `json:"path"`
		Kind string `json:"kind"`
	} `json:"imports"`
}

type EsbuildOutput struct {
	Bytes        int64                           `json:"bytes"`
	Inputs       map[string]EsbuildBytesInOutput `json:"inputs"`
	Imports      []EsbuildImport                 `json:"imports"`
	EntryPoint   string                          `json:"entryPoint"`
	CssBundle    string                          `json:"cssBundle"`
}

type EsbuildBytesInOutput struct {
	BytesInOutput int64 `json:"bytesInOutput"`
}

type EsbuildImport struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
}

type EsbuildParser struct{}

func (p *EsbuildParser) Name() string {
	return "esbuild"
}

func (p *EsbuildParser) Detect(sample []byte, distDir string) bool {
	// The esbuild-family signature is a top-level "inputs" map, which always
	// appears in the head sample and never in webpack/vite stats.
	return hasJSONKeys(sample, "inputs")
}

func (p *EsbuildParser) Parse(ctx context.Context, target core.Target) (*core.Bundle, error) {
	var meta EsbuildMetafile
	if err := decodeStats(ctx, target.StatsPath, &meta); err != nil {
		return nil, fmt.Errorf("parse esbuild metafile JSON: %w", err)
	}

	if len(meta.Outputs) == 0 {
		return nil, fmt.Errorf("esbuild metafile %q contains no outputs", target.StatsPath)
	}

	distDir := resolveDistDir(target.StatsPath, target.DistPath)

	bundle := core.NewBundle(core.Metadata{
		Bundler: "esbuild",
	})

	// Iterate outputs in sorted key order for deterministic chunk/module ordering
	sortedOutputs := make([]string, 0, len(meta.Outputs))
	for outPath := range meta.Outputs {
		sortedOutputs = append(sortedOutputs, outPath)
	}
	slices.Sort(sortedOutputs)

	// Process outputs as chunks
	for _, outPath := range sortedOutputs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		out := meta.Outputs[outPath]
		ext := strings.ToLower(filepath.Ext(outPath))
		if ext != ".js" && ext != ".mjs" && ext != ".css" {
			// Track as auxiliary asset
			gzBytes, estimated := gzipFor(distDir, outPath, out.Bytes)
			bundle.AddAsset(core.Asset{
				Path:          outPath,
				SizeBytes:     out.Bytes,
				GzipBytes:     gzBytes,
				GzipEstimated: estimated,
			})
			if estimated {
				bundle.GzipEstimated = true
			}
			continue
		}

		loadType := core.LoadTypeAsync
		if out.EntryPoint != "" {
			loadType = core.LoadTypeInitial
		}

		chunkID := filepath.Base(outPath)
		gzBytes, estimated := gzipFor(distDir, outPath, out.Bytes)
		chunk := core.Chunk{
			ID:            chunkID,
			Name:          chunkID,
			Path:          outPath,
			SizeBytes:     out.Bytes,
			GzipBytes:     gzBytes,
			GzipEstimated: estimated,
			Type:          loadType,
			Entry:         out.EntryPoint,
			ModuleIDs:     make([]string, 0, len(out.Inputs)),
		}
		if estimated {
			bundle.GzipEstimated = true
		}

		// Process modules in chunk (sorted for determinism). Module gzip is
		// source-level attribution and always ratio-estimated.
		inPaths := make([]string, 0, len(out.Inputs))
		for inPath := range out.Inputs {
			inPaths = append(inPaths, inPath)
		}
		slices.Sort(inPaths)
		for _, inPath := range inPaths {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			inBytes := out.Inputs[inPath]
			chunk.ModuleIDs = append(chunk.ModuleIDs, inPath)
			pkgName := extractPackageName(inPath)
			isApp := (pkgName == "")

			bundle.AddModule(core.Module{
				ID:        inPath,
				Package:   pkgName,
				SizeBytes: inBytes.BytesInOutput,
				GzipBytes: estimateGzip(inBytes.BytesInOutput, inPath),
				IsAppCode: isApp,
				ChunkIDs:  []string{chunkID},
			})
		}

		bundle.AddChunk(chunk)

		// If entrypoint, register it
		if out.EntryPoint != "" {
			bundle.AddEntrypoint(out.EntryPoint, core.Entrypoint{
				Name:             out.EntryPoint,
				InitialBytes:     out.Bytes,
				InitialGzipBytes: gzBytes,
				ChunkIDs:         []string{chunkID},
			})
		}
	}

	return bundle, nil
}

func extractPackageName(path string) string {
	idx := strings.Index(path, "node_modules/")
	if idx == -1 {
		return ""
	}
	sub := path[idx+len("node_modules/"):]
	parts := strings.Split(sub, "/")
	if len(parts) == 0 {
		return ""
	}
	if strings.HasPrefix(parts[0], "@") && len(parts) > 1 {
		return parts[0] + "/" + parts[1]
	}
	return parts[0]
}