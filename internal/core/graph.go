package core

// ImportEdge is one module-to-module import.
type ImportEdge struct {
	To      string
	Dynamic bool
}

// ImportGraph is the module import graph recovered from a build's metafile.
// It is only available for bundlers whose stats expose import edges and is
// never serialised.
type ImportGraph struct {
	Roots []string
	Edges map[string][]ImportEdge
}

// StaticReach returns every module statically reachable from the roots.
// Edges for which skip returns true are ignored.
func (g *ImportGraph) StaticReach(skip func(from, to string) bool) map[string]bool {
	seen := make(map[string]bool, len(g.Edges))
	queue := make([]string, 0, len(g.Roots))
	for _, r := range g.Roots {
		if !seen[r] {
			seen[r] = true
			queue = append(queue, r)
		}
	}
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		for _, e := range g.Edges[curr] {
			if e.Dynamic || seen[e.To] {
				continue
			}
			if skip != nil && skip(curr, e.To) {
				continue
			}
			seen[e.To] = true
			queue = append(queue, e.To)
		}
	}
	return seen
}

// InitialChunkSet returns the IDs of chunks loaded at startup.
func (b *Bundle) InitialChunkSet() map[string]bool {
	set := make(map[string]bool)
	for _, c := range b.Chunks {
		if c.Type == LoadTypeInitial {
			set[c.ID] = true
		}
	}
	return set
}

// ModuleInitialBytes returns the bytes a module contributes to the given
// initial chunks.
func ModuleInitialBytes(m Module, initialChunks map[string]bool) int64 {
	if len(m.ChunkBytes) > 0 {
		var n int64
		for cid, bytes := range m.ChunkBytes {
			if initialChunks[cid] {
				n += bytes
			}
		}
		return n
	}
	for _, cid := range m.ChunkIDs {
		if initialChunks[cid] {
			return m.SizeBytes
		}
	}
	return 0
}
