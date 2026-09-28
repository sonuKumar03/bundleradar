package diff

import (
	"cmp"
	"math"
	"slices"
	"sort"
	"strings"

	"github.com/sonuKumar03/bundleradar/internal/core"
)

// Options specifies parameters for bundle diffing.
type Options struct {
	DriftThreshold int64 // Threshold in bytes under which module delta is classified as micro-drift
}

// DiffSummary summarizes bundle sizes before, after, and delta.
type DiffSummary struct {
	BaseInitialBytes  int64 `json:"baseInitialBytes"`
	HeadInitialBytes  int64 `json:"headInitialBytes"`
	InitialDeltaBytes int64 `json:"initialDeltaBytes"`

	BaseLazyBytes  int64 `json:"baseLazyBytes"`
	HeadLazyBytes  int64 `json:"headLazyBytes"`
	LazyDeltaBytes int64 `json:"lazyDeltaBytes"`

	BaseTotalBytes  int64 `json:"baseTotalBytes"`
	HeadTotalBytes  int64 `json:"headTotalBytes"`
	TotalDeltaBytes int64 `json:"totalDeltaBytes"`
}

// BundleDiff contains deltas between base and current bundles.
type BundleDiff struct {
	Summary           DiffSummary                `json:"summary"`
	Entrypoints       map[string]EntrypointDelta `json:"entrypoints"`
	Packages          []PackageDelta             `json:"packages"`
	UnchangedPackages []PackageDelta             `json:"unchangedPackages,omitempty"`
	AddedChunks       []core.Chunk               `json:"addedChunks"`
	RemovedChunks     []core.Chunk               `json:"removedChunks"`
	MicroDriftBytes   int64                      `json:"microDriftBytes"`
	Attributions      []Attribution              `json:"attributions"`
}

// EntrypointDelta captures size changes for a specific entrypoint.
type EntrypointDelta struct {
	Name             string `json:"name"`
	InitialDelta     int64  `json:"initialDeltaBytes"`
	InitialGzipDelta int64  `json:"initialGzipDeltaBytes"`
	AsyncDelta       int64  `json:"asyncDeltaBytes"`
}

// PackageDelta captures size changes for an npm package.
type PackageDelta struct {
	Name       string   `json:"name"`
	DeltaBytes int64    `json:"deltaBytes"`
	GzipDelta  int64    `json:"gzipDeltaBytes"`
	BaseBytes  int64    `json:"baseBytes"`
	CurrBytes  int64    `json:"currBytes"`
	Status     string   `json:"status"` // "ELIMINATED", "REDUCED", "REGRESSED", "ADDED", "UNCHANGED"
	ChunkNames []string `json:"chunkNames,omitempty"`
	ImportPath string   `json:"importPath,omitempty"`
}

// Package movement statuses shared by all diff consumers (CLI, server UI, MCP).
const (
	StatusEliminated = "ELIMINATED"
	StatusReduced    = "REDUCED"
	StatusRegressed  = "REGRESSED"
	StatusAdded      = "ADDED"
	StatusUnchanged  = "UNCHANGED"
)

// Status classifies package movement between two byte measurements. It is the
// single source of truth for diff status semantics across all consumers.
func Status(baseBytes, targetBytes int64) string {
	switch {
	case baseBytes > 0 && targetBytes == 0:
		return StatusEliminated
	case baseBytes == 0 && targetBytes > 0:
		return StatusAdded
	case targetBytes < baseBytes:
		return StatusReduced
	case targetBytes > baseBytes:
		return StatusRegressed
	default:
		return StatusUnchanged
	}
}

// Attribution attributes a regression to a source module and reason.
type Attribution struct {
	SourceFile string `json:"sourceFile"`
	Reason     string `json:"reason"`
	DeltaBytes int64  `json:"deltaBytes"`
}

// Calculate computes comprehensive differences between a base bundle and a current bundle.
func Calculate(base, current *core.Bundle, opts Options) *BundleDiff {
	res := &BundleDiff{
		Entrypoints:       make(map[string]EntrypointDelta),
		Packages:          make([]PackageDelta, 0),
		UnchangedPackages: make([]PackageDelta, 0),
		AddedChunks:       make([]core.Chunk, 0),
		RemovedChunks:     make([]core.Chunk, 0),
		Attributions:      make([]Attribution, 0),
	}

	if base == nil && current == nil {
		return res
	}

	if base == nil {
		base = core.NewBundle(core.Metadata{})
	}
	if current == nil {
		current = core.NewBundle(core.Metadata{})
	}

	// 1. High-level Summary
	baseInit := base.TotalInitialBytes()
	baseLazy := base.TotalAsyncBytes()
	currInit := current.TotalInitialBytes()
	currLazy := current.TotalAsyncBytes()

	res.Summary = DiffSummary{
		BaseInitialBytes:  baseInit,
		HeadInitialBytes:  currInit,
		InitialDeltaBytes: currInit - baseInit,

		BaseLazyBytes:  baseLazy,
		HeadLazyBytes:  currLazy,
		LazyDeltaBytes: currLazy - baseLazy,

		BaseTotalBytes:  baseInit + baseLazy,
		HeadTotalBytes:  currInit + currLazy,
		TotalDeltaBytes: (currInit + currLazy) - (baseInit + baseLazy),
	}

	// 2. Entrypoint deltas
	for _, name := range unionKeys(base.Entrypoints, current.Entrypoints) {
		baseEP := base.Entrypoints[name]
		currEP := current.Entrypoints[name]

		res.Entrypoints[name] = EntrypointDelta{
			Name:             name,
			InitialDelta:     currEP.InitialBytes - baseEP.InitialBytes,
			InitialGzipDelta: currEP.InitialGzipBytes - baseEP.InitialGzipBytes,
			AsyncDelta:       currEP.AsyncBytes - baseEP.AsyncBytes,
		}
	}

	// 3. Chunks added / removed
	baseChunkMap := make(map[string]core.Chunk)
	for _, c := range base.Chunks {
		baseChunkMap[c.ID] = c
	}
	currChunkMap := make(map[string]core.Chunk)
	for _, c := range current.Chunks {
		currChunkMap[c.ID] = c
		if _, exists := baseChunkMap[c.ID]; !exists {
			res.AddedChunks = append(res.AddedChunks, c)
		}
	}
	for _, c := range base.Chunks {
		if _, exists := currChunkMap[c.ID]; !exists {
			res.RemovedChunks = append(res.RemovedChunks, c)
		}
	}

	// 4. Package deltas & Attribution
	basePkgMap := make(map[string]core.PackageContribution)
	for _, p := range base.TopPackages(0) {
		basePkgMap[p.Name] = p
	}
	currPkgMap := make(map[string]core.PackageContribution)
	for _, p := range current.TopPackages(0) {
		currPkgMap[p.Name] = p
	}

	// Map modules to package names for chunk and ingress path resolution
	currPkgMods := make(map[string][]core.Module)
	for _, m := range current.Modules {
		if m.Package != "" {
			currPkgMods[m.Package] = append(currPkgMods[m.Package], m)
		}
	}
	basePkgMods := make(map[string][]core.Module)
	for _, m := range base.Modules {
		if m.Package != "" {
			basePkgMods[m.Package] = append(basePkgMods[m.Package], m)
		}
	}

	for _, name := range unionKeys(basePkgMap, currPkgMap) {
		bSize := basePkgMap[name].SizeBytes
		bGzip := basePkgMap[name].GzipBytes
		cSize := currPkgMap[name].SizeBytes
		cGzip := currPkgMap[name].GzipBytes

		delta := cSize - bSize

		// Determine emitting chunks and ingress import path
		chunkSet := make(map[string]bool)
		var bestPath []string
		mods := currPkgMods[name]
		if len(mods) == 0 {
			mods = basePkgMods[name]
		}
		for _, m := range mods {
			for _, cid := range m.ChunkIDs {
				chunkSet[cid] = true
			}
			if len(m.IngressPaths) > 0 {
				if len(bestPath) == 0 || len(m.IngressPaths) < len(bestPath) {
					bestPath = m.IngressPaths
				}
			}
		}
		chunkNames := make([]string, 0, len(chunkSet))
		for c := range chunkSet {
			chunkNames = append(chunkNames, c)
		}
		sort.Strings(chunkNames)

		importPathStr := ""
		if len(bestPath) > 0 {
			importPathStr = strings.Join(bestPath, " → ")
		}

		pkgDelta := PackageDelta{
			Name:       name,
			DeltaBytes: delta,
			GzipDelta:  cGzip - bGzip,
			BaseBytes:  bSize,
			CurrBytes:  cSize,
			Status:     Status(bSize, cSize),
			ChunkNames: chunkNames,
			ImportPath: importPathStr,
		}

		if delta != 0 {
			res.Packages = append(res.Packages, pkgDelta)
			if delta > 0 && (opts.DriftThreshold == 0 || delta >= opts.DriftThreshold) {
				res.Attributions = append(res.Attributions, Attribution{
					SourceFile: name,
					Reason:     importPathStr,
					DeltaBytes: delta,
				})
			}
		} else {
			res.UnchangedPackages = append(res.UnchangedPackages, pkgDelta)
		}
	}

	slices.SortFunc(res.Packages, func(a, b PackageDelta) int {
		absA := math.Abs(float64(a.DeltaBytes))
		absB := math.Abs(float64(b.DeltaBytes))
		if absA != absB {
			return cmp.Compare(absB, absA)
		}
		return strings.Compare(a.Name, b.Name)
	})
	slices.SortFunc(res.UnchangedPackages, func(a, b PackageDelta) int {
		if a.CurrBytes != b.CurrBytes {
			return cmp.Compare(b.CurrBytes, a.CurrBytes)
		}
		return strings.Compare(a.Name, b.Name)
	})

	// 5. Module diff & Micro-drift
	baseModMap := make(map[string]core.Module)
	for _, m := range base.Modules {
		baseModMap[m.ID] = m
	}
	currModMap := make(map[string]core.Module)
	for _, m := range current.Modules {
		currModMap[m.ID] = m
	}

	for _, id := range unionKeys(baseModMap, currModMap) {
		delta := currModMap[id].SizeBytes - baseModMap[id].SizeBytes
		if delta == 0 {
			continue
		}
		if opts.DriftThreshold > 0 && delta > 0 && delta < opts.DriftThreshold {
			res.MicroDriftBytes += delta
		}
	}

	return res
}

func unionKeys[M ~map[string]V, V any](a, b M) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	for k := range a {
		seen[k] = struct{}{}
	}
	for k := range b {
		seen[k] = struct{}{}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	return keys
}
