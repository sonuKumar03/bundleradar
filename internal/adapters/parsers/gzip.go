package parsers

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
)

// MIME ratio buckets for gzip estimation when the emitted file is unavailable.
// These are averages across representative assets of each type; real ratios
// vary, which is why real gzip is preferred whenever the file exists on disk.
const (
	ratioJS       = 0.25
	ratioText     = 0.35
	ratioJSON     = 0.30
	ratioBinary   = 0.0 // already compressed (images, fonts, wasm): gzip adds ~0
	ratioFallback = 0.30
)

// mimeRatio returns the average gzip savings ratio for a file extension.
func mimeRatio(path string) float64 {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".js", ".mjs", ".cjs":
		return ratioJS
	case ".css", ".html", ".htm", ".svg", ".txt", ".xml":
		return ratioText
	case ".json", ".map":
		return ratioJSON
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".avif", ".ico", ".woff", ".woff2", ".ttf", ".eot", ".wasm", ".mp4", ".webm", ".mp3":
		return ratioBinary
	default:
		return ratioFallback
	}
}

// RealGzip compresses data with standard gzip and returns the compressed length.
// When the file is not found on disk, realGzip returns ok=false and the caller
// falls back to a per-MIME ratio estimate.
func realGzip(path string) (int64, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	var buf bytes.Buffer
	gw, err := gzip.NewWriterLevel(&buf, gzip.DefaultCompression)
	if err != nil {
		return 0, false
	}
	if _, err := gw.Write(data); err != nil {
		return 0, false
	}
	if err := gw.Close(); err != nil {
		return 0, false
	}
	return int64(buf.Len()), true
}

// estimateGzip approximates gzip size using per-MIME ratios. It is only used
// when the emitted artifact is not on disk. Callers that use it must set the
// resulting GzipEstimated flag so consumers know the number is approximate.
func estimateGzip(rawBytes int64, path string) int64 {
	return int64(float64(rawBytes) * mimeRatio(path))
}

// resolveDistDir returns the directory that contains emitted build artifacts.
// Prefers the explicitly supplied dist path, then falls back to the stats
// file's own directory (vite manifests and webpack stats commonly live beside
// their outputs), then to a "browser" subdirectory (Angular application builder).
func resolveDistDir(statsPath, distPath string) string {
	if distPath != "" {
		if isAngularBrowserDir(distPath) {
			return filepath.Join(distPath, "browser")
		}
		return distPath
	}
	if statsPath != "" {
		dir := filepath.Dir(statsPath)
		if isAngularBrowserDir(dir) {
			return filepath.Join(dir, "browser")
		}
		return dir
	}
	return ""
}

func isAngularBrowserDir(dir string) bool {
	fi, err := os.Stat(filepath.Join(dir, "browser", "index.html"))
	return err == nil && !fi.IsDir()
}

// gzipFor attempts real gzip of path relative to distDir; on any failure it
// returns a per-MIME estimate and estimated=true.
func gzipFor(distDir, relPath string, rawBytes int64) (bytes int64, estimated bool) {
	if distDir != "" {
		if n, ok := realGzip(filepath.Join(distDir, relPath)); ok {
			return n, false
		}
	}
	return estimateGzip(rawBytes, relPath), true
}
