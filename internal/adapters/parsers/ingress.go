package parsers

import "github.com/sonuKumar03/bundleradar/internal/core"

// isStaticImportKind reports edges that load a module at startup. CommonJS
// require() calls are static: esbuild inlines them into the importing chunk.
func isStaticImportKind(kind string) bool {
	return kind == "import-statement" || kind == "require-call"
}

// buildEsbuildIngressPaths computes an import chain from the roots to every
// reachable module in two regions:
//
//   - the initial region follows static imports from rootInputs, ignoring
//     edges into lazyOnly inputs (esbuild keeps unused imports in the metafile,
//     so those edges are not real paths);
//   - the lazy region starts at dynamic-import targets of the initial region and
//     at lazyRoots (lazy route entries, which have no input-level import from
//     main), and follows every edge.
//
// A module's path comes from the initial region when it has one, so a trail
// explains startup cost first and only falls back to lazy chains.
func buildEsbuildIngressPaths(inputs map[string]EsbuildInput, rootInputs, lazyRoots []string, lazyOnly map[string]bool) map[string][]string {
	initial, order := bfsIngress(inputs, seedPaths(rootInputs), func(kind, target string) bool {
		return isStaticImportKind(kind) && !lazyOnly[target]
	})

	seeds := make([][]string, 0)
	seen := make(map[string]bool)
	for _, from := range order {
		for _, imp := range inputs[from].Imports {
			if imp.Kind != "dynamic-import" || imp.Path == "" || seen[imp.Path] {
				continue
			}
			if _, inInitial := initial[imp.Path]; inInitial {
				continue
			}
			seen[imp.Path] = true
			seeds = append(seeds, append(append([]string(nil), initial[from]...), imp.Path))
		}
	}
	for _, r := range lazyRoots {
		if !seen[r] {
			seen[r] = true
			seeds = append(seeds, []string{r})
		}
	}
	lazy, _ := bfsIngress(inputs, seeds, func(kind, _ string) bool {
		return isStaticImportKind(kind) || kind == "dynamic-import"
	})

	for id, path := range lazy {
		if _, ok := initial[id]; !ok {
			initial[id] = path
		}
	}
	return initial
}

func seedPaths(roots []string) [][]string {
	seeds := make([][]string, len(roots))
	for i, r := range roots {
		seeds[i] = []string{r}
	}
	return seeds
}

// bfsIngress runs a breadth-first search from the seed paths over the edges
// accepted by follow, returning the first path found to each module and the
// order modules were reached in.
func bfsIngress(inputs map[string]EsbuildInput, seeds [][]string, follow func(kind, target string) bool) (map[string][]string, []string) {
	paths := make(map[string][]string)
	order := make([]string, 0, len(seeds))
	for _, seed := range seeds {
		id := seed[len(seed)-1]
		if _, ok := paths[id]; !ok {
			paths[id] = seed
			order = append(order, id)
		}
	}
	for i := 0; i < len(order); i++ {
		curr := order[i]
		for _, imp := range inputs[curr].Imports {
			if imp.Path == "" || !follow(imp.Kind, imp.Path) {
				continue
			}
			if _, ok := paths[imp.Path]; ok {
				continue
			}
			next := make([]string, len(paths[curr])+1)
			copy(next, paths[curr])
			next[len(next)-1] = imp.Path
			paths[imp.Path] = next
			order = append(order, imp.Path)
		}
	}
	return paths, order
}

// buildImportGraph records the module import edges of an esbuild metafile.
func buildImportGraph(inputs map[string]EsbuildInput, rootInputs []string) *core.ImportGraph {
	g := &core.ImportGraph{Roots: rootInputs, Edges: make(map[string][]core.ImportEdge, len(inputs))}
	for id, in := range inputs {
		for _, imp := range in.Imports {
			if imp.Path == "" || (!isStaticImportKind(imp.Kind) && imp.Kind != "dynamic-import") {
				continue
			}
			g.Edges[id] = append(g.Edges[id], core.ImportEdge{To: imp.Path, Dynamic: imp.Kind == "dynamic-import"})
		}
	}
	return g
}
