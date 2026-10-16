package core_test

import (
	"strings"
	"testing"

	"github.com/sonuKumar03/bundleradar/internal/core"
)

func consistentBundle() *core.Bundle {
	b := core.NewBundle(core.Metadata{Bundler: "angular"})
	b.AddChunk(core.Chunk{ID: "main.js", Path: "main.js", SizeBytes: 100, Type: core.LoadTypeInitial})
	b.AddChunk(core.Chunk{ID: "lazy.js", Path: "lazy.js", SizeBytes: 400, Type: core.LoadTypeAsync})
	b.AddChunk(core.Chunk{ID: "styles.css", Path: "styles.css", SizeBytes: 50, Type: core.LoadTypeInitial})
	b.AddEntrypoint("main", core.Entrypoint{Name: "main", InitialBytes: 100, AsyncBytes: 400, ChunkIDs: []string{"main.js"}})
	b.AddModule(core.Module{ID: "src/a.ts", SizeBytes: 100, ChunkIDs: []string{"main.js"}, ChunkBytes: map[string]int64{"main.js": 100}})
	return b
}

func TestCheckInvariantsAcceptsConsistentBundle(t *testing.T) {
	if w := consistentBundle().CheckInvariants(); len(w) != 0 {
		t.Fatalf("unexpected warnings: %v", w)
	}
}

func TestCheckInvariantsFlagsInconsistentBundles(t *testing.T) {
	tests := map[string]struct {
		mutate func(*core.Bundle)
		want   string
	}{
		"no initial roots": {
			func(b *core.Bundle) { b.Entrypoints["main"] = core.Entrypoint{Name: "main", ChunkIDs: []string{"main.js"}} },
			"no initial JavaScript was identified",
		},
		"lazy bytes missing": {
			func(b *core.Bundle) {
				b.Entrypoints["main"] = core.Entrypoint{Name: "main", InitialBytes: 100, ChunkIDs: []string{"main.js"}}
			},
			"do not add up",
		},
		"initial differs from initial chunks": {
			func(b *core.Bundle) {
				b.Entrypoints["main"] = core.Entrypoint{Name: "main", InitialBytes: 90, AsyncBytes: 410, ChunkIDs: []string{"main.js"}}
			},
			"does not match",
		},
		"unknown chunk": {
			func(b *core.Bundle) {
				b.Entrypoints["main"] = core.Entrypoint{Name: "main", InitialBytes: 100, AsyncBytes: 400, ChunkIDs: []string{"main.js", "ghost.js"}}
			},
			`unknown chunk "ghost.js"`,
		},
		"modules exceed chunk": {
			func(b *core.Bundle) {
				b.AddModule(core.Module{ID: "src/b.ts", SizeBytes: 60, ChunkIDs: []string{"main.js"}, ChunkBytes: map[string]int64{"main.js": 60}})
			},
			"more than the chunk's",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			b := consistentBundle()
			tc.mutate(b)
			w := strings.Join(b.CheckInvariants(), "\n")
			if !strings.Contains(w, tc.want) {
				t.Fatalf("warnings = %q, want one containing %q", w, tc.want)
			}
		})
	}
}

func TestCheckInvariantsSkipsSumChecksForMultipleEntrypoints(t *testing.T) {
	b := consistentBundle()
	b.AddEntrypoint("other", core.Entrypoint{Name: "other", InitialBytes: 100, ChunkIDs: []string{"main.js"}})
	if w := b.CheckInvariants(); len(w) != 0 {
		t.Fatalf("shared chunks across entrypoints must not trigger sum checks: %v", w)
	}
}
