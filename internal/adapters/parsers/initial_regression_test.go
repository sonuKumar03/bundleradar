package parsers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/sonuKumar03/bundleradar/internal/adapters/parsers"
	"github.com/sonuKumar03/bundleradar/internal/adapters/reporters"
	"github.com/sonuKumar03/bundleradar/internal/core"
	"github.com/sonuKumar03/bundleradar/internal/core/policy"
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

func TestAngularWhyFollowsDynamicImportIngress(t *testing.T) {
	stats := writeStats(t, "stats.json", `{"inputs":{"src/main.ts":{"imports":[{"path":"src/lazy.ts","kind":"dynamic-import"}]},"src/lazy.ts":{"imports":[{"path":"node_modules/heavy/index.js","kind":"import-statement"}]},"node_modules/heavy/index.js":{}},"outputs":{"main.js":{"bytes":100,"entryPoint":"src/main.ts","imports":[{"path":"lazy.js","kind":"dynamic-import"}],"inputs":{"src/main.ts":{"bytesInOutput":100}}},"lazy.js":{"bytes":200,"inputs":{"node_modules/heavy/index.js":{"bytesInOutput":200}}}}}`)
	bundle, err := (&parsers.AngularParser{}).Parse(context.Background(), core.Target{StatsPath: stats})
	if err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := reporters.NewWhy("heavy", "json").Render(context.Background(), &output, bundle); err != nil {
		t.Fatal(err)
	}
	var result struct {
		LazyBytes int64 `json:"lazyBytes"`
		Chains    []struct {
			Initial bool     `json:"initial"`
			Path    []string `json:"path"`
		} `json:"chains"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.LazyBytes != 200 || len(result.Chains) != 1 || result.Chains[0].Initial {
		t.Fatalf("lazy contribution/load type = %+v, want 200 bytes in a lazy output", result)
	}
	wantPath := []string{"src/main.ts", "src/lazy.ts", "node_modules/heavy/index.js"}
	if !slices.Equal(result.Chains[0].Path, wantPath) {
		t.Fatalf("lazy ingress path = %v, want %v", result.Chains[0].Path, wantPath)
	}
}

func TestAngularDuplicateVersionsUseInstalledPackageMetadata(t *testing.T) {
	stats := writeStats(t, "stats.json", `{"inputs":{},"outputs":{"main.js":{"bytes":100,"entryPoint":"src/main.ts","inputs":{"node_modules/pkg/index.js":{"bytesInOutput":100}}},"lazy.js":{"bytes":200,"inputs":{"node_modules/parent/node_modules/pkg/index.js":{"bytesInOutput":200}}}}}`)
	for dir, version := range map[string]string{
		"node_modules/pkg":                     "1.2.3",
		"node_modules/parent/node_modules/pkg": "2.0.0",
	} {
		packageDir := filepath.Join(filepath.Dir(stats), dir)
		if err := os.MkdirAll(packageDir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(packageDir, "package.json"), []byte(`{"version":"`+version+`"}`), 0600); err != nil {
			t.Fatal(err)
		}
	}

	bundle, err := (&parsers.AngularParser{}).Parse(context.Background(), core.Target{StatsPath: stats})
	if err != nil {
		t.Fatal(err)
	}
	versions := map[string]bool{}
	for _, module := range bundle.Modules {
		if module.Package == "pkg" {
			versions[module.Version] = true
		}
	}
	if !versions["1.2.3"] || !versions["2.0.0"] || len(versions) != 2 {
		t.Fatalf("installed package versions = %v, want 1.2.3 and 2.0.0", versions)
	}
	result := policy.Evaluate(bundle, nil, policy.Policy{DetectDuplicatePkgs: true})
	if result.Passed || len(result.Violations) != 1 || result.Violations[0].Rule != "DUPLICATE_PACKAGE_VER" {
		t.Fatalf("duplicate-version result = %+v, want DUPLICATE_PACKAGE_VER", result)
	}
}

func TestAngularSharedModuleCountsBytesInEachOutput(t *testing.T) {
	stats := writeStats(t, "stats.json", `{"inputs":{},"outputs":{"main.js":{"bytes":100,"entryPoint":"src/main.ts","inputs":{"node_modules/lodash/index.js":{"bytesInOutput":100}}},"lazy.js":{"bytes":900,"inputs":{"node_modules/lodash/index.js":{"bytesInOutput":900}}}}}`)
	bundle, err := (&parsers.AngularParser{}).Parse(context.Background(), core.Target{StatsPath: stats})
	if err != nil {
		t.Fatal(err)
	}
	for _, mod := range bundle.Modules {
		if mod.ID == "node_modules/lodash/index.js" {
			if mod.SizeBytes != 1000 {
				t.Fatalf("shared module bytes = %d, want 1000 across both outputs", mod.SizeBytes)
			}
			if mod.ChunkBytes["main.js"] != 100 || mod.ChunkBytes["lazy.js"] != 900 {
				t.Fatalf("shared module chunk bytes = %v, want main=100 lazy=900", mod.ChunkBytes)
			}
			return
		}
	}
	t.Fatal("shared lodash module missing from parsed bundle")
}
