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

func TestAngularRootsFallBackWhenIndexHTMLMatchesNothing(t *testing.T) {
	stats := writeStats(t, "stats.json", `{"inputs":{"node_modules/@angular/core/index.js":{}},"outputs":{"main-AAA.js":{"bytes":100,"entryPoint":"src/main.ts","inputs":{"src/main.ts":{"bytesInOutput":100}}},"lazy-BBB.js":{"bytes":900,"entryPoint":"src/app/lazy.module.ts","inputs":{"src/app/lazy.module.ts":{"bytesInOutput":900}}}}}`)
	dir := filepath.Dir(stats)
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(`<script src="SD_FE_DEPLOY_URLmain.js"></script><link href="https://cdn.example.com/font.css">`), 0600); err != nil {
		t.Fatal(err)
	}

	bundle, err := (&parsers.AngularParser{}).Parse(context.Background(), core.Target{StatsPath: stats})
	if err != nil {
		t.Fatal(err)
	}
	if got := bundle.Entrypoints["main"].InitialBytes; got != 100 {
		t.Fatalf("initial bytes = %d, want 100 (main only, lazy route excluded)", got)
	}
	if got := bundle.Entrypoints["main"].AsyncBytes; got != 900 {
		t.Fatalf("async bytes = %d, want 900", got)
	}
}

func TestAngularIngressPrefersStaticPathOverShorterDynamicOne(t *testing.T) {
	stats := writeStats(t, "stats.json", `{"inputs":{"src/main.ts":{"imports":[{"path":"src/lazy.ts","kind":"dynamic-import"},{"path":"src/a.ts","kind":"import-statement"}]},"src/lazy.ts":{"imports":[{"path":"node_modules/heavy/index.js","kind":"import-statement"}]},"src/a.ts":{"imports":[{"path":"src/b.ts","kind":"import-statement"}]},"src/b.ts":{"imports":[{"path":"node_modules/heavy/index.js","kind":"import-statement"}]},"node_modules/heavy/index.js":{}},"outputs":{"main.js":{"bytes":300,"entryPoint":"src/main.ts","inputs":{"src/main.ts":{"bytesInOutput":100},"src/a.ts":{"bytesInOutput":50},"src/b.ts":{"bytesInOutput":50},"node_modules/heavy/index.js":{"bytesInOutput":100}}}}}`)
	bundle, err := (&parsers.AngularParser{}).Parse(context.Background(), core.Target{StatsPath: stats})
	if err != nil {
		t.Fatal(err)
	}
	mod, ok := bundle.FindModule("node_modules/heavy/index.js")
	if !ok {
		t.Fatal("heavy module missing")
	}
	want := []string{"src/main.ts", "src/a.ts", "src/b.ts", "node_modules/heavy/index.js"}
	if !slices.Equal(mod.IngressPaths, want) {
		t.Fatalf("ingress path = %v, want static path %v", mod.IngressPaths, want)
	}
}

func TestEsbuildModulesCarryInstalledVersions(t *testing.T) {
	stats := writeStats(t, "meta.json", `{"inputs":{"src/index.ts":{"imports":[{"path":"node_modules/foo/index.js","kind":"import-statement"}]},"node_modules/foo/index.js":{}},"outputs":{"out.js":{"bytes":100,"entryPoint":"src/index.ts","inputs":{"src/index.ts":{"bytesInOutput":40},"node_modules/foo/index.js":{"bytesInOutput":60}}}}}`)
	pkgDir := filepath.Join(filepath.Dir(stats), "node_modules", "foo")
	if err := os.MkdirAll(pkgDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "package.json"), []byte(`{"version":"1.2.3"}`), 0600); err != nil {
		t.Fatal(err)
	}

	bundle, err := (&parsers.EsbuildParser{}).Parse(context.Background(), core.Target{StatsPath: stats})
	if err != nil {
		t.Fatal(err)
	}
	mod, ok := bundle.FindModule("node_modules/foo/index.js")
	if !ok || mod.Version != "1.2.3" {
		t.Fatalf("module = %+v, want version 1.2.3", mod)
	}
}

func TestWhyReportsRetainedSizeAndImporters(t *testing.T) {
	stats := writeStats(t, "stats.json", `{"inputs":{"src/main.ts":{"imports":[{"path":"src/a.ts","kind":"import-statement"},{"path":"src/b.ts","kind":"import-statement"},{"path":"src/c.ts","kind":"import-statement"}]},"src/a.ts":{"imports":[{"path":"node_modules/pdf/index.js","kind":"import-statement"}]},"src/b.ts":{"imports":[{"path":"node_modules/pdf/index.js","kind":"import-statement"}]},"src/c.ts":{"imports":[{"path":"node_modules/solo/index.js","kind":"import-statement"}]},"node_modules/pdf/index.js":{"imports":[{"path":"node_modules/pdfdep/index.js","kind":"import-statement"}]},"node_modules/pdfdep/index.js":{},"node_modules/solo/index.js":{}},"outputs":{"main.js":{"bytes":1000,"entryPoint":"src/main.ts","inputs":{"src/main.ts":{"bytesInOutput":10},"src/a.ts":{"bytesInOutput":10},"src/b.ts":{"bytesInOutput":10},"src/c.ts":{"bytesInOutput":10},"node_modules/pdf/index.js":{"bytesInOutput":300},"node_modules/pdfdep/index.js":{"bytesInOutput":200},"node_modules/solo/index.js":{"bytesInOutput":50}}}}}`)
	bundle, err := (&parsers.AngularParser{}).Parse(context.Background(), core.Target{StatsPath: stats})
	if err != nil {
		t.Fatal(err)
	}
	trace := func(target string) (retained int64, savings map[string]int64) {
		var out bytes.Buffer
		if err := reporters.NewWhy(target, "json").Render(context.Background(), &out, bundle); err != nil {
			t.Fatal(err)
		}
		var res struct {
			Retained  int64 `json:"retainedInitialBytes"`
			Importers []struct {
				Module      string `json:"module"`
				FirstParty  bool   `json:"firstParty"`
				SavingBytes int64  `json:"savingBytes"`
			} `json:"importers"`
		}
		if err := json.Unmarshal(out.Bytes(), &res); err != nil {
			t.Fatal(err)
		}
		savings = map[string]int64{}
		for _, imp := range res.Importers {
			if !imp.FirstParty {
				t.Fatalf("%s importer %s should be first-party", target, imp.Module)
			}
			savings[imp.Module] = imp.SavingBytes
		}
		return res.Retained, savings
	}

	retained, savings := trace("pdf")
	if retained != 500 {
		t.Fatalf("pdf retained = %d, want 500 (pdf + its dependency)", retained)
	}
	if len(savings) != 2 || savings["src/a.ts"] != 0 || savings["src/b.ts"] != 0 {
		t.Fatalf("pdf importer savings = %v, want a.ts and b.ts saving 0 each", savings)
	}

	retained, savings = trace("solo")
	if retained != 50 || savings["src/c.ts"] != 50 {
		t.Fatalf("solo retained=%d savings=%v, want 50 and c.ts saving 50", retained, savings)
	}
}

func TestWhyRetainedSizeFollowsCommonJSRequireCalls(t *testing.T) {
	stats := writeStats(t, "stats.json", `{"inputs":{"src/main.ts":{"imports":[{"path":"node_modules/cjs/index.js","kind":"import-statement"}]},"node_modules/cjs/index.js":{"imports":[{"path":"node_modules/cjs/impl.js","kind":"require-call"}]},"node_modules/cjs/impl.js":{"imports":[{"path":"node_modules/cjs/data.json","kind":"require-call"}]},"node_modules/cjs/data.json":{}},"outputs":{"main.js":{"bytes":1000,"entryPoint":"src/main.ts","inputs":{"src/main.ts":{"bytesInOutput":10},"node_modules/cjs/index.js":{"bytesInOutput":5},"node_modules/cjs/impl.js":{"bytesInOutput":85},"node_modules/cjs/data.json":{"bytesInOutput":900}}}}}`)
	bundle, err := (&parsers.AngularParser{}).Parse(context.Background(), core.Target{StatsPath: stats})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := reporters.NewWhy("cjs", "json").Render(context.Background(), &out, bundle); err != nil {
		t.Fatal(err)
	}
	var res struct {
		Retained int64 `json:"retainedInitialBytes"`
	}
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.Retained != 990 {
		t.Fatalf("retained = %d, want 990 (whole CommonJS package incl. data.json)", res.Retained)
	}
}

func TestAngularRootsAreExactFileNamesOutsideNodeModules(t *testing.T) {
	stats := writeStats(t, "stats.json", `{"inputs":{"src/main.ts":{},"libs/x/domain.ts":{},"node_modules/pkg/entries/main.js":{}},"outputs":{"main.js":{"bytes":100,"entryPoint":"src/main.ts","inputs":{"src/main.ts":{"bytesInOutput":40},"libs/x/domain.ts":{"bytesInOutput":30},"node_modules/pkg/entries/main.js":{"bytesInOutput":30}}}}}`)
	bundle, err := (&parsers.AngularParser{}).Parse(context.Background(), core.Target{StatsPath: stats})
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Graph == nil {
		t.Fatal("expected an import graph")
	}
	for _, r := range bundle.Graph.Roots {
		if r != "src/main.ts" {
			t.Fatalf("unexpected root %q; only src/main.ts should be a root", r)
		}
	}
}

func TestWhyIgnoresStaticEdgesIntoLazyOnlyModules(t *testing.T) {
	stats := writeStats(t, "stats.json", `{"inputs":{"src/main.ts":{"imports":[{"path":"src/lazy.ts","kind":"import-statement"}]},"src/lazy.ts":{"imports":[{"path":"node_modules/pkg/index.js","kind":"import-statement"}]},"node_modules/pkg/index.js":{}},"outputs":{"main.js":{"bytes":100,"entryPoint":"src/main.ts","inputs":{"src/main.ts":{"bytesInOutput":100}}},"lazy.js":{"bytes":500,"inputs":{"src/lazy.ts":{"bytesInOutput":100},"node_modules/pkg/index.js":{"bytesInOutput":400}}}}}`)
	bundle, err := (&parsers.AngularParser{}).Parse(context.Background(), core.Target{StatsPath: stats})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := reporters.NewWhy("pkg", "json").Render(context.Background(), &out, bundle); err != nil {
		t.Fatal(err)
	}
	var res struct {
		Retained  int64             `json:"retainedInitialBytes"`
		Importers []json.RawMessage `json:"importers"`
	}
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.Retained != 0 || len(res.Importers) != 0 {
		t.Fatalf("lazy-only package: retained=%d importers=%d, want 0 and none", res.Retained, len(res.Importers))
	}
}

func TestInitialBytesExcludeStylesheets(t *testing.T) {
	angular := writeStats(t, "stats.json", `{"inputs":{"node_modules/@angular/core/index.js":{}},"outputs":{"main.js":{"bytes":100,"entryPoint":"src/main.ts","inputs":{"src/main.ts":{"bytesInOutput":100}}},"styles.css":{"bytes":40,"entryPoint":"angular:styles/global:styles","inputs":{}}}}`)
	bundle, err := (&parsers.AngularParser{}).Parse(context.Background(), core.Target{StatsPath: angular})
	if err != nil {
		t.Fatal(err)
	}
	if got := bundle.Entrypoints["main"].InitialBytes; got != 100 {
		t.Fatalf("angular initial = %d, want 100 (JS only)", got)
	}
	if got := bundle.TotalCSSBytes(); got != 40 {
		t.Fatalf("angular CSS = %d, want 40 reported separately", got)
	}

	esb := writeStats(t, "meta.json", `{"inputs":{"src/index.ts":{},"src/app.css":{}},"outputs":{"out/index.js":{"bytes":70,"entryPoint":"src/index.ts","inputs":{"src/index.ts":{"bytesInOutput":70}}},"out/app.css":{"bytes":30,"entryPoint":"src/app.css","inputs":{"src/app.css":{"bytesInOutput":30}}}}}`)
	bundle, err = (&parsers.EsbuildParser{}).Parse(context.Background(), core.Target{StatsPath: esb})
	if err != nil {
		t.Fatal(err)
	}
	if got := bundle.TotalInitialBytes(); got != 70 {
		t.Fatalf("esbuild initial = %d, want 70 (CSS entry must not count)", got)
	}
	if got := bundle.TotalJSBytes(); got != 70 {
		t.Fatalf("esbuild JS total = %d, want 70", got)
	}
	if got := bundle.TotalCSSBytes(); got != 30 {
		t.Fatalf("esbuild CSS = %d, want 30", got)
	}
}

func TestEsbuildAttributesLazyChunksToTheEntry(t *testing.T) {
	stats := writeStats(t, "meta.json", `{"inputs":{"src/main.ts":{},"src/lazy.ts":{}},"outputs":{"out/main.js":{"bytes":100,"entryPoint":"src/main.ts","imports":[{"path":"out/chunk-LAZY.js","kind":"dynamic-import"}],"inputs":{"src/main.ts":{"bytesInOutput":100}}},"out/chunk-LAZY.js":{"bytes":400,"imports":[],"inputs":{"src/lazy.ts":{"bytesInOutput":400}}},"out/app.css":{"bytes":70,"imports":[],"inputs":{}}}}`)
	bundle, err := (&parsers.EsbuildParser{}).Parse(context.Background(), core.Target{StatsPath: stats})
	if err != nil {
		t.Fatal(err)
	}
	ep := bundle.Entrypoints["src/main.ts"]
	if ep.InitialBytes != 100 || ep.AsyncBytes != 400 {
		t.Fatalf("initial=%d lazy=%d, want 100 and 400 (CSS excluded)", ep.InitialBytes, ep.AsyncBytes)
	}
	if w := bundle.CheckInvariants(); len(w) != 0 {
		t.Fatalf("a correctly parsed bundle must pass its own invariants: %v", w)
	}
}
