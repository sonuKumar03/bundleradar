package parsers

import "github.com/sonuKumar03/bundleradar/internal/core"

// isStaticImportKind reports edges that load a module at startup. CommonJS
// require() calls are static: esbuild inlines them into the importing chunk.
func isStaticImportKind(kind string) bool {
	return kind == "import-statement" || kind == "require-call"
}

// buildEsbuildIngressPaths computes an import chain from the roots to every
// reachable module. Static ("import-statement") chains are preferred so a
// path explains why a module ships in the initial bundle; only modules that
// are unreachable statically fall back to chains through "dynamic-import".
func buildEsbuildIngressPaths(inputs map[string]EsbuildInput, rootInputs []string) map[string][]string {
	ingressPaths := make(map[string][]string)
	order := make([]string, 0, len(rootInputs))
	for _, r := range rootInputs {
		if _, seen := ingressPaths[r]; !seen {
			ingressPaths[r] = []string{r}
			order = append(order, r)
		}
	}

	expand := func(allowDynamic bool) {
		queue := append([]string(nil), order...)
		for len(queue) > 0 {
			curr := queue[0]
			queue = queue[1:]
			currPath := ingressPaths[curr]
			inData, ok := inputs[curr]
			if !ok {
				continue
			}
			for _, imp := range inData.Imports {
				if !isStaticImportKind(imp.Kind) && (!allowDynamic || imp.Kind != "dynamic-import") {
					continue
				}
				target := imp.Path
				if target == "" {
					continue
				}
				if _, seen := ingressPaths[target]; seen {
					continue
				}
				nextPath := make([]string, len(currPath)+1)
				copy(nextPath, currPath)
				nextPath[len(currPath)] = target
				ingressPaths[target] = nextPath
				order = append(order, target)
				queue = append(queue, target)
			}
		}
	}

	expand(false)
	expand(true)
	return ingressPaths
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
