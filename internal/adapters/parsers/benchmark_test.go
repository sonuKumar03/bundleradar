package parsers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/sonuKumar03/bundleradar/internal/core"
)

// BenchmarkParseLargeMetafile measures memory/time on a synthetic metafile
// sized to resemble real-world Angular/esbuild outputs (hundreds of MB with
// tens of thousands of modules). Regressions here indicate the streaming
// decoder path is loading too much into memory.
func BenchmarkParseLargeMetafile(b *testing.B) {
	tmp := b.TempDir()
	statsPath := filepath.Join(tmp, "large-meta.json")
	if err := writeLargeMetafile(statsPath, 500, 50); err != nil {
		b.Fatalf("write large metafile: %v", err)
	}

	parser := &EsbuildParser{}
	target := core.Target{StatsPath: statsPath}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bundle, err := parser.Parse(context.Background(), target)
		if err != nil {
			b.Fatalf("parse: %v", err)
		}
		if len(bundle.Modules) == 0 {
			b.Fatalf("expected modules")
		}
	}
}

// writeLargeMetafile generates a metafile with chunks*outPkgCount outputs, each
// referencing shared inputs, approximating ~ (chunks * inputsPerChunk) modules.
func writeLargeMetafile(path string, chunks, inputsPerChunk int) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintf(f, `{"inputs":{`)
	first := true
	for i := 0; i < chunks*inputsPerChunk; i++ {
		if !first {
			fmt.Fprint(f, ",")
		}
		first = false
		fmt.Fprintf(f, `"src/mod%d.ts":{"bytes":%d,"imports":[]}`, i, 1500)
	}
	fmt.Fprintf(f, `},"outputs":{`)
	first = true
	for c := 0; c < chunks; c++ {
		if !first {
			fmt.Fprint(f, ",")
		}
		first = false
		fmt.Fprintf(f, `"out/chunk%d.js":{"bytes":%d,"inputs":{`, c, 20000)
		firstIn := true
		for m := 0; m < inputsPerChunk; m++ {
			if !firstIn {
				fmt.Fprint(f, ",")
			}
			firstIn = false
			modIdx := c*inputsPerChunk + m
			fmt.Fprintf(f, `"src/mod%d.ts":{"bytesInOutput":%d}`, modIdx, 1200)
		}
		fmt.Fprintf(f, `},"imports":[],"entryPoint":"%s"}}`, entryPointName(c))
	}
	fmt.Fprint(f, `}}`)
	return f.Close()
}

func entryPointName(c int) string {
	if c == 0 {
		return "main.ts"
	}
	return ""
}