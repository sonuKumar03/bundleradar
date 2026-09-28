package parsers

import "testing"

func TestDetectStructureOverSubstring(t *testing.T) {
	esbuildMeta := `{"inputs":{"a.ts":{"bytes":1}},"outputs":{"a.js":{"bytes":1}}}`
	viteManifest := `{"index.html":{"file":"assets/index.js","isEntry":true}}`
	webpackStats := `{"assetsByChunkName":{},"chunks":[{"id":0,"files":["main.js"]}],"modules":[]}`

	tests := []struct {
		name    string
		sample  string
		esbuild bool
		vite    bool
		webpack bool
	}{
		{"esbuild metafile", esbuildMeta, true, false, false},
		{"vite manifest", viteManifest, false, true, false},
		{"webpack stats", webpackStats, false, false, true},
		// Adversarial: keywords mentioned inside string VALUES must not trigger
		{"webpack value mentions inputs", `{"chunks":[],"modules":[{"name":"contains \"inputs\"","size":1}]}`, false, false, true},
		{"vite value mentions modules", `{"x":{"file":"a.js","isEntry":true,"note":"modules inside a value"}}`, false, true, false},
		{"esbuild value mentions isEntry", `{"inputs":{"x.ts":{"bytes":1,"label":"isEntry"}},"outputs":{}}`, true, false, false},
		// Truncated sample (head-only) still detects via early keys
		{"truncated esbuild", `{"inputs":{"a":`, true, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := []byte(tt.sample)
			if got := (&EsbuildParser{}).Detect(s, ""); got != tt.esbuild {
				t.Errorf("esbuild Detect = %v, want %v", got, tt.esbuild)
			}
			if got := (&ViteParser{}).Detect(s, ""); got != tt.vite {
				t.Errorf("vite Detect = %v, want %v", got, tt.vite)
			}
			if got := (&WebpackParser{}).Detect(s, ""); got != tt.webpack {
				t.Errorf("webpack Detect = %v, want %v", got, tt.webpack)
			}
		})
	}
}
