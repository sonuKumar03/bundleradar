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

func TestAngularInitialFilesFollowIndexHTML(t *testing.T) {
	stats := writeStats(t, "stats.json", `{"inputs":{"node_modules/@angular/core/index.js":{}},"outputs":{"main.js":{"bytes":100,"entryPoint":"src/main.ts","inputs":{"src/main.ts":{"bytesInOutput":100}}},"scripts.js":{"bytes":45,"entryPoint":"angular:script/global:scripts.js","inputs":{}},"optional.css":{"bytes":21,"entryPoint":"angular:styles/global:optional","inputs":{}}}}`)
	browser := filepath.Join(filepath.Dir(stats), "browser")
	if err := os.MkdirAll(browser, 0700); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"index.html":   `<script src="scripts.js"></script><script src="main.js"></script>`,
		"main.js":      "main payload",
		"scripts.js":   "global script",
		"optional.css": "unused style",
	} {
		if err := os.WriteFile(filepath.Join(browser, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}

	bundle, err := (&parsers.AngularParser{}).Parse(context.Background(), core.Target{StatsPath: stats})
	if err != nil {
		t.Fatal(err)
	}
	if got := bundle.Entrypoints["main"].InitialBytes; got != 145 {
		t.Fatalf("initial bytes = %d, want injected JS only (145)", got)
	}
	if got := bundle.Entrypoints["main"].AsyncBytes; got != 0 {
		t.Fatalf("async JS bytes = %d, want 0", got)
	}
	if bundle.GzipEstimated {
		t.Fatal("gzip sizes should use emitted files under browser/")
	}
}
