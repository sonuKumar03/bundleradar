package parsers_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sonuKumar03/bundleradar/internal/adapters/parsers"
	"github.com/sonuKumar03/bundleradar/internal/core"
)

func writeStats(t *testing.T, name, data string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestEsbuildStaticImportsCountAsInitial(t *testing.T) {
	stats := writeStats(t, "stats.json", `{"inputs":{},"outputs":{"main.js":{"bytes":100,"entryPoint":"src/main.ts","imports":[{"path":"vendor.js","kind":"import-statement"}],"inputs":{}},"vendor.js":{"bytes":900,"imports":[],"inputs":{}},"lazy.js":{"bytes":200,"imports":[],"inputs":{}}}}`)
	b, err := (&parsers.EsbuildParser{}).Parse(context.Background(), core.Target{StatsPath: stats})
	if err != nil {
		t.Fatal(err)
	}
	if got := b.Entrypoints["src/main.ts"].InitialBytes; got != 1000 {
		t.Fatalf("initial JS = %d, want 1000", got)
	}
	if ch, _ := b.FindChunk("vendor.js"); ch.Type != core.LoadTypeInitial {
		t.Fatalf("static vendor type = %q, want initial", ch.Type)
	}
}

func TestViteStaticImportsCountAsInitial(t *testing.T) {
	dir := t.TempDir()
	for name, size := range map[string]int{"main.js": 100, "vendor.js": 900, "lazy.js": 200} {
		if err := os.WriteFile(filepath.Join(dir, name), make([]byte, size), 0600); err != nil {
			t.Fatal(err)
		}
	}
	stats := filepath.Join(dir, "manifest.json")
	data := `{"src/main.ts":{"file":"main.js","isEntry":true,"imports":["_vendor.js"],"dynamicImports":["src/lazy.ts"]},"_vendor.js":{"file":"vendor.js"},"src/lazy.ts":{"file":"lazy.js","isDynamicEntry":true}}`
	if err := os.WriteFile(stats, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	b, err := (&parsers.ViteParser{}).Parse(context.Background(), core.Target{StatsPath: stats, DistPath: dir})
	if err != nil {
		t.Fatal(err)
	}
	ep := b.Entrypoints["src/main.ts"]
	if ep.InitialBytes != 1000 || ep.AsyncBytes != 200 {
		t.Fatalf("entrypoint initial=%d async=%d, want 1000/200", ep.InitialBytes, ep.AsyncBytes)
	}
	if ch, _ := b.FindChunk("vendor.js"); ch.Type != core.LoadTypeInitial {
		t.Fatalf("static vendor type = %q, want initial", ch.Type)
	}
}
