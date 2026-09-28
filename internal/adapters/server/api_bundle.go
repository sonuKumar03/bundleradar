package server

import (
	"cmp"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strings"

	"github.com/sonuKumar03/bundleradar/internal/core"
	"github.com/sonuKumar03/bundleradar/pkg/bundleradar"
)

// BundleDTO represents the normalized bundle AST payload returned by the studio API.
type BundleDTO struct {
	Bundler       string                   `json:"bundler,omitempty"`
	StatsPath     string                   `json:"statsPath"`
	Entrypoints   map[string]EntrypointDTO `json:"entrypoints"`
	Chunks        []ChunkDTO               `json:"chunks"`
	TopPackages   []PackageDTO             `json:"topPackages"`
	TotalBytes    int64                    `json:"totalBytes"`
	GzipEstimated bool                     `json:"gzipEstimated,omitempty"`
}

// EntrypointDTO represents entrypoint sizes and constituent chunks.
type EntrypointDTO struct {
	InitialBytes     int64    `json:"initialBytes"`
	InitialGzipBytes int64    `json:"initialGzipBytes"`
	AsyncBytes       int64    `json:"asyncBytes"`
	ChunkIds         []string `json:"chunkIds"`
}

// ChunkDTO represents an emitted chunk artifact.
type ChunkDTO struct {
	Name          string `json:"name"`
	Type          string `json:"type"` // "initial" or "async"
	SizeBytes     int64  `json:"sizeBytes"`
	GzipBytes     int64  `json:"gzipBytes"`
	GzipEstimated bool   `json:"gzipEstimated,omitempty"`
}

// PackageDTO aggregates module metrics and BFS ingress attribution for a single package.
type PackageDTO struct {
	Name         string   `json:"name"`
	SizeBytes    int64    `json:"sizeBytes"`
	GzipBytes    int64    `json:"gzipBytes"`
	InitialBytes int64    `json:"initialBytes"`
	AsyncBytes   int64    `json:"asyncBytes"`
	Chunks       []string `json:"chunks"`
	IngressPath  string   `json:"ingressPath"`
}

func (s *Server) handleGetBundle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	buildID := r.URL.Query().Get("build")
	if buildID != "" && buildID != "latest" {
		cp := s.GetCheckpoint(buildID)
		if cp != nil && cp.Bundle != nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(cp.Bundle)
			return
		}
	}

	bundle := s.Bundle()
	statsPath := s.statsPath

	if bundle == nil && statsPath != "" {
		scanned, err := s.client.Scan(r.Context(), bundleradar.ScanOptions{
			StatsPath: statsPath,
		})
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": fmt.Sprintf("failed to parse bundle stats: %v", err),
			})
			return
		}
		s.SetBundle(scanned)
		bundle = scanned
		if s.CheckpointCount() == 0 {
			s.RecordCheckpoint(scanned, "Build #1 (Initial Baseline)")
		}
	}

	if bundle == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "No stats.json has been scanned yet",
		})
		return
	}

	dto := s.BundleToDTO(bundle)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(dto)
}

// BundleToDTO converts a core.Bundle model into the wire DTO representation.
func (s *Server) BundleToDTO(bundle *core.Bundle) *BundleDTO {
	if bundle == nil {
		return nil
	}

	var totalBytes int64
	chunks := make([]ChunkDTO, 0, len(bundle.Chunks))
	for _, ch := range bundle.Chunks {
		totalBytes += ch.SizeBytes
		chunks = append(chunks, ChunkDTO{
			Name:          ch.Name,
			Type:          string(ch.Type),
			SizeBytes:     ch.SizeBytes,
			GzipBytes:     ch.GzipBytes,
			GzipEstimated: ch.GzipEstimated,
		})
	}
	if totalBytes == 0 {
		totalBytes = bundle.TotalInitialBytes() + bundle.TotalAsyncBytes()
	}

	entrypoints := make(map[string]EntrypointDTO, len(bundle.Entrypoints))
	for name, ep := range bundle.Entrypoints {
		entrypoints[name] = EntrypointDTO{
			InitialBytes:     ep.InitialBytes,
			InitialGzipBytes: ep.InitialGzipBytes,
			AsyncBytes:       ep.AsyncBytes,
			ChunkIds:         ep.ChunkIDs,
		}
	}

	// Build map of chunk load types
	chunkTypeMap := make(map[string]core.LoadType)
	for _, ch := range bundle.Chunks {
		chunkTypeMap[ch.ID] = ch.Type
		chunkTypeMap[ch.Name] = ch.Type
	}

	// Aggregate and sort packages
	pkgMap := make(map[string]*PackageDTO)
	bestPaths := make(map[string][]string)

	for _, mod := range bundle.Modules {
		pkgName := mod.Package
		if pkgName == "" {
			pkgName = "(application code)"
		}
		p, exists := pkgMap[pkgName]
		if !exists {
			p = &PackageDTO{
				Name:   pkgName,
				Chunks: make([]string, 0),
			}
			pkgMap[pkgName] = p
		}
		p.SizeBytes += mod.SizeBytes
		p.GzipBytes += mod.GzipBytes

		hasInitial := false
		hasAsync := false
		for _, cid := range mod.ChunkIDs {
			if !slices.Contains(p.Chunks, cid) {
				p.Chunks = append(p.Chunks, cid)
			}
			switch chunkTypeMap[cid] {
			case core.LoadTypeInitial:
				hasInitial = true
			case core.LoadTypeAsync:
				hasAsync = true
			}
		}

		if hasInitial {
			p.InitialBytes += mod.SizeBytes
		}
		if hasAsync {
			p.AsyncBytes += mod.SizeBytes
		}

		if pkgName != "(application code)" && len(mod.IngressPaths) > 0 {
			bp := bestPaths[pkgName]
			if len(bp) == 0 || len(mod.IngressPaths) < len(bp) {
				bestPaths[pkgName] = mod.IngressPaths
			}
		}
	}

	topPackages := make([]PackageDTO, 0, len(pkgMap))
	for pkgName, p := range pkgMap {
		sort.Strings(p.Chunks)
		if bp, ok := bestPaths[pkgName]; ok && len(bp) > 0 {
			p.IngressPath = strings.Join(bp, " → ")
		}
		topPackages = append(topPackages, *p)
	}

	slices.SortFunc(topPackages, func(a, b PackageDTO) int {
		if a.SizeBytes != b.SizeBytes {
			return cmp.Compare(b.SizeBytes, a.SizeBytes)
		}
		return strings.Compare(a.Name, b.Name)
	})

	return &BundleDTO{
		Bundler:       bundle.Metadata.Bundler,
		StatsPath:     s.statsPath,
		Entrypoints:   entrypoints,
		Chunks:        chunks,
		TopPackages:   topPackages,
		TotalBytes:    totalBytes,
		GzipEstimated: bundle.GzipEstimated,
	}
}
