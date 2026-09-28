package parsers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
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
	return bytes.Contains(sample, []byte(`"inputs"`)) && (bytes.Contains(sample, []byte(`"outputs"`)) || bytes.Contains(sample, []byte(`"bytes":`)))
}

func (p *EsbuildParser) Parse(ctx context.Context, target core.Target) (*core.Bundle, error) {
	data, err := os.ReadFile(target.StatsPath)
	if err != nil {
		return nil, fmt.Errorf("read esbuild metafile %q: %w", target.StatsPath, err)
	}

	var meta EsbuildMetafile
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parse esbuild metafile JSON: %w", err)
	}

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
		out := meta.Outputs[outPath]
		ext := strings.ToLower(filepath.Ext(outPath))
		if ext != ".js" && ext != ".mjs" && ext != ".css" {
			// Track as auxiliary asset
			bundle.AddAsset(core.Asset{
				Path:      outPath,
				SizeBytes: out.Bytes,
				GzipBytes: estimateGzip(out.Bytes),
			})
			continue
		}

		loadType := core.LoadTypeAsync
		if out.EntryPoint != "" {
			loadType = core.LoadTypeInitial
		}

		chunkID := filepath.Base(outPath)
		chunk := core.Chunk{
			ID:        chunkID,
			Name:      chunkID,
			Path:      outPath,
			SizeBytes: out.Bytes,
			GzipBytes: estimateGzip(out.Bytes),
			Type:      loadType,
			Entry:     out.EntryPoint,
			ModuleIDs: make([]string, 0, len(out.Inputs)),
		}

		// Process modules in chunk (sorted for determinism)
		inPaths := make([]string, 0, len(out.Inputs))
		for inPath := range out.Inputs {
			inPaths = append(inPaths, inPath)
		}
		slices.Sort(inPaths)
		for _, inPath := range inPaths {
			inBytes := out.Inputs[inPath]
			chunk.ModuleIDs = append(chunk.ModuleIDs, inPath)
			pkgName := extractPackageName(inPath)
			isApp := (pkgName == "")

			bundle.AddModule(core.Module{
				ID:        inPath,
				Package:   pkgName,
				SizeBytes: inBytes.BytesInOutput,
				GzipBytes: estimateGzip(inBytes.BytesInOutput),
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
				InitialGzipBytes: estimateGzip(out.Bytes),
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

func estimateGzip(rawBytes int64) int64 {
	// Standard gzip compression ratio approximation for JS/CSS bundles
	return int64(float64(rawBytes) * 0.30)
}
