package reporters_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sonuKumar03/bundleradar/internal/adapters/reporters"
	"github.com/sonuKumar03/bundleradar/internal/core"
)

func whyFixture() *core.Bundle {
	b := core.NewBundle(core.Metadata{Bundler: "esbuild"})
	b.AddChunk(core.Chunk{ID: "main.js", Name: "main.js", SizeBytes: 6000, Type: core.LoadTypeInitial})
	b.AddChunk(core.Chunk{ID: "lazy.js", Name: "lazy.js", SizeBytes: 4000, Type: core.LoadTypeAsync})
	b.AddModule(core.Module{
		ID:           "node_modules/lodash/lodash.js",
		Package:      "lodash",
		SizeBytes:    6000,
		ChunkIDs:     []string{"main.js"},
		IngressPaths: []string{"src/main.ts", "node_modules/lodash/lodash.js"},
	})
	b.AddModule(core.Module{
		ID:           "node_modules/lodash/another.js",
		Package:      "lodash",
		SizeBytes:    4000,
		ChunkIDs:     []string{"lazy.js"},
		IngressPaths: []string{"src/report.ts", "node_modules/lodash/another.js"},
	})
	return b
}

func TestWhyReporter_JSON(t *testing.T) {
	var buf bytes.Buffer
	rep := reporters.NewWhy("lodash", "json")
	if err := rep.Render(context.Background(), &buf, whyFixture()); err != nil {
		t.Fatalf("render: %v", err)
	}
	var res struct {
		Target       string `json:"target"`
		Found        bool   `json:"found"`
		InitialBytes int64  `json:"initialBytes"`
		LazyBytes    int64  `json:"lazyBytes"`
	}
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if res.Target != "lodash" || !res.Found {
		t.Errorf("target/found wrong: %+v", res)
	}
	if res.InitialBytes != 6000 || res.LazyBytes != 4000 {
		t.Errorf("bytes wrong: init=%d lazy=%d", res.InitialBytes, res.LazyBytes)
	}
}

func TestWhyReporter_Terminal(t *testing.T) {
	var buf bytes.Buffer
	rep := reporters.NewWhy("lodash", "")
	if err := rep.Render(context.Background(), &buf, whyFixture()); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"IMPORT TRACE", "main.js [initial]", "lazy.js [lazy]", "Ingress Trail"} {
		if !strings.Contains(out, want) {
			t.Errorf("terminal output missing %q:\n%s", want, out)
		}
	}
}

func TestWhyReporter_NotFound(t *testing.T) {
	var buf bytes.Buffer
	rep := reporters.NewWhy("nonexistent", "")
	if err := rep.Render(context.Background(), &buf, whyFixture()); err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(buf.String(), "not found") {
		t.Errorf("expected not-found message, got %q", buf.String())
	}
}