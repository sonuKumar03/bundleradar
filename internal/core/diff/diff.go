package diff

import (
	"cmp"
	"math"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/sonuKumar03/bundleradar/internal/core"
)

// Options specifies parameters for bundle diffing.
type Options struct {
	DriftThreshold int64 // Threshold in bytes under which module delta is classified as micro-drift
	// IncludeUnchanged lists every unchanged package in the result. Off by
	// default because it grows the output with the size of the whole bundle.
	IncludeUnchanged bool
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
	UnchangedCount    int                        `json:"unchangedCount"`
	FirstParty        []PackageDelta             `json:"firstParty,omitempty"`
	AddedChunks       []core.Chunk               `json:"addedChunks"`
	RemovedChunks     []core.Chunk               `json:"removedChunks"`
	ChangedChunks     []ChunkDelta               `json:"changedChunks,omitempty"`
	MicroDriftBytes   int64                      `json:"microDriftBytes"`
	Attributions      []Attribution              `json:"attributions"`
}

// ChunkDelta describes a chunk that exists in both builds under a different
// content-hashed file name (or the same name with different bytes).
type ChunkDelta struct {
	Name       string `json:"name"`
	BaseID     string `json:"baseId"`
	HeadID     string `json:"headId"`
	BaseBytes  int64  `json:"baseBytes"`
	HeadBytes  int64  `json:"headBytes"`
	DeltaBytes int64  `json:"deltaBytes"`
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
	// InitialDeltaBytes is the change in bytes this package contributes to
	// initial chunks. It moves even when DeltaBytes is zero, e.g. when a
	// static import pulls a lazy package into the initial bundle.
	BaseInitialBytes int64 `json:"baseInitialBytes,omitempty"`
	CurrInitialBytes int64 `json:"currInitialBytes,omitempty"`
	InitialDelta     int64 `json:"initialDeltaBytes,omitempty"`
}

// Package movement statuses shared by all diff consumers (CLI, server UI, MCP).
const (
	StatusEliminated = "ELIMINATED"
	StatusReduced    = "REDUCED"
	StatusRegressed  = "REGRESSED"
	StatusAdded      = "ADDED"
	StatusUnchanged  = "UNCHANGED"
	// StatusPromoted: same total size, but more of it now ships in initial chunks.
	StatusPromoted = "PROMOTED"
	// StatusDeferred: same total size, but less of it ships in initial chunks.
	StatusDeferred = "DEFERRED"
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
	baseTotal, currTotal := base.TotalJSBytes(), current.TotalJSBytes()

	res.Summary = DiffSummary{
		BaseInitialBytes:  baseInit,
		HeadInitialBytes:  currInit,
		InitialDeltaBytes: currInit - baseInit,

		BaseLazyBytes:  baseLazy,
		HeadLazyBytes:  currLazy,
		LazyDeltaBytes: currLazy - baseLazy,

		BaseTotalBytes:  baseTotal,
		HeadTotalBytes:  currTotal,
		TotalDeltaBytes: currTotal - baseTotal,
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

	// 3. Chunks added / removed / changed
	res.AddedChunks, res.RemovedChunks, res.ChangedChunks = diffChunks(base, current)

	// 4. Package deltas & Attribution — single pass per bundle builds both
	// contribution totals and module groupings to avoid redundant iteration.
	basePkgMap, basePkgMods, basePkgInit := aggregatePackages(base)
	currPkgMap, currPkgMods, currPkgInit := aggregatePackages(current)

	for _, name := range unionKeys(basePkgMap, currPkgMap) {
		bSize := basePkgMap[name].SizeBytes
		bGzip := basePkgMap[name].GzipBytes
		cSize := currPkgMap[name].SizeBytes
		cGzip := currPkgMap[name].GzipBytes

		delta := cSize - bSize
		bInit := basePkgInit[name]
		cInit := currPkgInit[name]
		initDelta := cInit - bInit

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

			BaseInitialBytes: bInit,
			CurrInitialBytes: cInit,
			InitialDelta:     initDelta,
		}
		if delta == 0 {
			switch {
			case initDelta > 0:
				pkgDelta.Status = StatusPromoted
			case initDelta < 0:
				pkgDelta.Status = StatusDeferred
			}
		}

		if delta != 0 || initDelta != 0 {
			res.Packages = append(res.Packages, pkgDelta)
			// A regression is any growth in total size or in what ships initially.
			regression := max(delta, initDelta)
			if regression > 0 && (opts.DriftThreshold == 0 || regression >= opts.DriftThreshold) {
				res.Attributions = append(res.Attributions, Attribution{
					SourceFile: name,
					Reason:     importPathStr,
					DeltaBytes: regression,
				})
			}
		} else {
			res.UnchangedCount++
			if opts.IncludeUnchanged {
				res.UnchangedPackages = append(res.UnchangedPackages, pkgDelta)
			}
		}
	}

	res.FirstParty = diffFirstParty(base, current)

	slices.SortFunc(res.Packages, func(a, b PackageDelta) int {
		absA := max(math.Abs(float64(a.DeltaBytes)), math.Abs(float64(a.InitialDelta)))
		absB := max(math.Abs(float64(b.DeltaBytes)), math.Abs(float64(b.InitialDelta)))
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

func aggregatePackages(b *core.Bundle) (map[string]core.PackageContribution, map[string][]core.Module, map[string]int64) {
	pkgMap := make(map[string]core.PackageContribution)
	pkgMods := make(map[string][]core.Module)
	pkgInitial := make(map[string]int64)
	initialChunks := b.InitialChunkSet()
	for _, m := range b.Modules {
		if m.IsAppCode || m.Package == "" {
			continue
		}
		contrib := pkgMap[m.Package]
		contrib.Name = m.Package
		contrib.SizeBytes += m.SizeBytes
		contrib.GzipBytes += m.GzipBytes
		contrib.ModuleCount++
		pkgMap[m.Package] = contrib
		pkgMods[m.Package] = append(pkgMods[m.Package], m)
		pkgInitial[m.Package] += core.ModuleInitialBytes(m, initialChunks)
	}
	return pkgMap, pkgMods, pkgInitial
}

// firstPartyGroup maps a first-party module path to its workspace project
// (apps/<name>, libs/<name>, packages/<name>) or its top-level directory.
func firstPartyGroup(id string) string {
	id = strings.TrimPrefix(filepath.ToSlash(id), "./")
	if strings.Contains(id, ":") {
		return ""
	}
	parts := strings.Split(id, "/")
	if len(parts) >= 2 && (parts[0] == "apps" || parts[0] == "libs" || parts[0] == "packages") {
		return parts[0] + "/" + parts[1]
	}
	if len(parts) >= 2 {
		return parts[0]
	}
	return "(root)"
}

func aggregateFirstParty(b *core.Bundle) (map[string]int64, map[string]int64, map[string]core.PackageContribution) {
	sizes := make(map[string]int64)
	initial := make(map[string]int64)
	contribs := make(map[string]core.PackageContribution)
	initialChunks := b.InitialChunkSet()
	for _, m := range b.Modules {
		if !m.IsAppCode {
			continue
		}
		g := firstPartyGroup(m.ID)
		if g == "" {
			continue
		}
		sizes[g] += m.SizeBytes
		initial[g] += core.ModuleInitialBytes(m, initialChunks)
		c := contribs[g]
		c.Name = g
		c.GzipBytes += m.GzipBytes
		contribs[g] = c
	}
	return sizes, initial, contribs
}

func diffFirstParty(base, current *core.Bundle) []PackageDelta {
	bSizes, bInit, bC := aggregateFirstParty(base)
	cSizes, cInit, cC := aggregateFirstParty(current)
	out := make([]PackageDelta, 0)
	for _, g := range unionKeys(bSizes, cSizes) {
		delta := cSizes[g] - bSizes[g]
		initDelta := cInit[g] - bInit[g]
		if delta == 0 && initDelta == 0 {
			continue
		}
		pd := PackageDelta{
			Name:             g,
			DeltaBytes:       delta,
			GzipDelta:        cC[g].GzipBytes - bC[g].GzipBytes,
			BaseBytes:        bSizes[g],
			CurrBytes:        cSizes[g],
			Status:           Status(bSizes[g], cSizes[g]),
			BaseInitialBytes: bInit[g],
			CurrInitialBytes: cInit[g],
			InitialDelta:     initDelta,
		}
		if delta == 0 && initDelta > 0 {
			pd.Status = StatusPromoted
		} else if delta == 0 && initDelta < 0 {
			pd.Status = StatusDeferred
		}
		out = append(out, pd)
	}
	slices.SortFunc(out, func(a, b PackageDelta) int {
		ma := max(math.Abs(float64(a.DeltaBytes)), math.Abs(float64(a.InitialDelta)))
		mb := max(math.Abs(float64(b.DeltaBytes)), math.Abs(float64(b.InitialDelta)))
		if ma != mb {
			return cmp.Compare(mb, ma)
		}
		return strings.Compare(a.Name, b.Name)
	})
	return out
}

var chunkHashSuffix = regexp.MustCompile(`[-.][A-Za-z0-9]{8}(\.(?:m?js|css))$`)

// stableChunkKey strips the content hash from an emitted file name so the same
// logical chunk can be recognised across builds.
func stableChunkKey(c core.Chunk) string {
	name := c.Name
	if name == "" {
		name = c.ID
	}
	return chunkHashSuffix.ReplaceAllString(filepath.Base(name), "$1")
}

func moduleOverlap(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	set := make(map[string]struct{}, len(a))
	for _, id := range a {
		set[id] = struct{}{}
	}
	inter := 0
	for _, id := range b {
		if _, ok := set[id]; ok {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	return float64(inter) / float64(union)
}

// diffChunks matches chunks by ID, then by hash-stripped name, then by module
// overlap, so a content change is reported as a changed chunk rather than as
// one removed and one added chunk. Added/removed chunks omit module lists.
func diffChunks(base, current *core.Bundle) (added, removed []core.Chunk, changed []ChunkDelta) {
	added, removed = make([]core.Chunk, 0), make([]core.Chunk, 0)
	baseByID := make(map[string]core.Chunk, len(base.Chunks))
	for _, c := range base.Chunks {
		baseByID[c.ID] = c
	}
	currByID := make(map[string]core.Chunk, len(current.Chunks))
	var addedC, removedC []core.Chunk
	for _, c := range current.Chunks {
		currByID[c.ID] = c
		if _, ok := baseByID[c.ID]; !ok {
			addedC = append(addedC, c)
		}
	}
	for _, c := range base.Chunks {
		if _, ok := currByID[c.ID]; !ok {
			removedC = append(removedC, c)
		}
	}

	pair := func(r, a core.Chunk) {
		changed = append(changed, ChunkDelta{
			Name: stableChunkKey(a), BaseID: r.ID, HeadID: a.ID,
			BaseBytes: r.SizeBytes, HeadBytes: a.SizeBytes, DeltaBytes: a.SizeBytes - r.SizeBytes,
		})
	}

	// Pass 1: unique hash-stripped name on both sides.
	aKeys, rKeys := make(map[string][]int), make(map[string][]int)
	for i, c := range addedC {
		aKeys[stableChunkKey(c)] = append(aKeys[stableChunkKey(c)], i)
	}
	for i, c := range removedC {
		rKeys[stableChunkKey(c)] = append(rKeys[stableChunkKey(c)], i)
	}
	usedA, usedR := make(map[int]bool), make(map[int]bool)
	for key, ai := range aKeys {
		if ri, ok := rKeys[key]; ok && len(ai) == 1 && len(ri) == 1 {
			pair(removedC[ri[0]], addedC[ai[0]])
			usedA[ai[0]], usedR[ri[0]] = true, true
		}
	}

	// Pass 2: remaining chunks paired by module overlap, best match first.
	type cand struct {
		a, r int
		s    float64
	}
	var cands []cand
	for i, a := range addedC {
		if usedA[i] {
			continue
		}
		for j, r := range removedC {
			if usedR[j] {
				continue
			}
			if s := moduleOverlap(a.ModuleIDs, r.ModuleIDs); s >= 0.5 {
				cands = append(cands, cand{i, j, s})
			}
		}
	}
	slices.SortFunc(cands, func(x, y cand) int { return cmp.Compare(y.s, x.s) })
	for _, c := range cands {
		if usedA[c.a] || usedR[c.r] {
			continue
		}
		pair(removedC[c.r], addedC[c.a])
		usedA[c.a], usedR[c.r] = true, true
	}

	slim := func(c core.Chunk) core.Chunk {
		c.ModuleIDs = nil
		return c
	}
	for i, c := range addedC {
		if !usedA[i] {
			added = append(added, slim(c))
		}
	}
	for i, c := range removedC {
		if !usedR[i] {
			removed = append(removed, slim(c))
		}
	}
	slices.SortFunc(changed, func(x, y ChunkDelta) int {
		ax, ay := math.Abs(float64(x.DeltaBytes)), math.Abs(float64(y.DeltaBytes))
		if ax != ay {
			return cmp.Compare(ay, ax)
		}
		return strings.Compare(x.Name, y.Name)
	})
	return added, removed, changed
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
