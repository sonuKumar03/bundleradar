package parsers

// buildEsbuildIngressPaths performs a BFS over esbuild metafile inputs to
// compute the shortest import chain from any root to every reachable module.
// It follows "import-statement" and "dynamic-import" edges.
func buildEsbuildIngressPaths(inputs map[string]EsbuildInput, rootInputs []string) map[string][]string {
	ingressPaths := make(map[string][]string)
	bfsQueue := make([]string, 0, len(rootInputs))
	for _, r := range rootInputs {
		if _, seen := ingressPaths[r]; !seen {
			ingressPaths[r] = []string{r}
			bfsQueue = append(bfsQueue, r)
		}
	}

	for len(bfsQueue) > 0 {
		curr := bfsQueue[0]
		bfsQueue = bfsQueue[1:]
		currPath := ingressPaths[curr]

		if inData, ok := inputs[curr]; ok {
			for _, imp := range inData.Imports {
				if imp.Kind != "import-statement" && imp.Kind != "dynamic-import" {
					continue
				}
				target := imp.Path
				if target == "" {
					continue
				}
				if _, seen := ingressPaths[target]; !seen {
					nextPath := make([]string, len(currPath)+1)
					copy(nextPath, currPath)
					nextPath[len(currPath)] = target
					ingressPaths[target] = nextPath
					bfsQueue = append(bfsQueue, target)
				}
			}
		}
	}

	return ingressPaths
}
