package updater_test

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sonuKumar03/bundleradar/internal/updater"
)

func tarGz(t *testing.T, name, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for fileName, body := range map[string]string{"README.md": "readme", name: content} {
		if err := tw.WriteHeader(&tar.Header{Name: fileName, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	_ = tw.Close()
	_ = gz.Close()
	return buf.Bytes()
}

func zipFile(t *testing.T, name, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte(content))
	_ = zw.Close()
	return buf.Bytes()
}

type fakeRelease struct {
	server *httptest.Server
	hits   atomic.Int32
}

// newFakeRelease serves a release for the given tag containing one archive.
func newFakeRelease(t *testing.T, tag, archiveName string, archive []byte, checksum string, withSums bool) *fakeRelease {
	t.Helper()
	f := &fakeRelease{}
	mux := http.NewServeMux()
	release := func(w http.ResponseWriter, r *http.Request) {
		f.hits.Add(1)
		assets := fmt.Sprintf(`{"name":%q,"browser_download_url":%q}`, archiveName, f.server.URL+"/dl/"+archiveName)
		if withSums {
			assets += fmt.Sprintf(`,{"name":"checksums.txt","browser_download_url":%q}`, f.server.URL+"/dl/checksums.txt")
		}
		fmt.Fprintf(w, `{"tag_name":%q,"assets":[%s]}`, tag, assets)
	}
	mux.HandleFunc("/releases/latest", release)
	mux.HandleFunc("/releases/tags/"+tag, release)
	mux.HandleFunc("/dl/"+archiveName, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(archive) })
	mux.HandleFunc("/dl/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s  %s\n", checksum, archiveName)
	})
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

func sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func client(f *fakeRelease) *updater.Client {
	c := updater.New("1.0.0")
	c.APIURL = f.server.URL + "/releases/latest"
	return c
}

func writeTarget(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "bundleradar")
	if err := os.WriteFile(p, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"2.6.0", "2.5.1", true},
		{"v2.5.2", "2.5.1", true},
		{"2.5.1", "2.5.1", false},
		{"2.5.0", "2.5.1", false},
		{"10.0.0", "9.9.9", true},
		{"garbage", "2.5.1", false},
		{"2.6.0", "dev", false},
	}
	for _, c := range cases {
		if got := updater.Newer(c.latest, c.current); got != c.want {
			t.Errorf("Newer(%q,%q) = %v, want %v", c.latest, c.current, got, c.want)
		}
	}
}

func TestAssetName(t *testing.T) {
	if got := updater.AssetName("v2.5.1", "darwin", "arm64"); got != "bundleradar_2.5.1_darwin_arm64.tar.gz" {
		t.Fatalf("got %q", got)
	}
	if got := updater.AssetName("2.5.1", "windows", "amd64"); got != "bundleradar_2.5.1_windows_amd64.zip" {
		t.Fatalf("got %q", got)
	}
}

func TestInstallReplacesBinaryAfterVerifyingChecksum(t *testing.T) {
	name := updater.AssetName("9.9.9", "linux", "amd64")
	archive := tarGz(t, "bundleradar", "NEW-BINARY")
	f := newFakeRelease(t, "v9.9.9", name, archive, sum(archive), true)
	c := client(f)

	rel, err := c.Release(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	target := writeTarget(t, "OLD-BINARY")
	if err := c.Install(context.Background(), rel, target, "linux", "amd64"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(target)
	if string(got) != "NEW-BINARY" {
		t.Fatalf("binary = %q, want NEW-BINARY", got)
	}
	if fi, _ := os.Stat(target); runtime.GOOS != "windows" && fi.Mode()&0o111 == 0 {
		t.Fatalf("installed binary is not executable: %v", fi.Mode())
	}
	if entries, _ := os.ReadDir(filepath.Dir(target)); len(entries) != 1 {
		t.Fatalf("temp files left behind: %v", entries)
	}
}

func TestInstallWindowsZip(t *testing.T) {
	name := updater.AssetName("9.9.9", "windows", "amd64")
	archive := zipFile(t, "bundleradar.exe", "NEW-EXE")
	f := newFakeRelease(t, "v9.9.9", name, archive, sum(archive), true)
	c := client(f)
	rel, err := c.Release(context.Background(), "v9.9.9")
	if err != nil {
		t.Fatal(err)
	}
	target := writeTarget(t, "OLD-EXE")
	if err := c.Install(context.Background(), rel, target, "windows", "amd64"); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(target); string(got) != "NEW-EXE" {
		t.Fatalf("binary = %q", got)
	}
	if _, err := os.Stat(target + ".old"); !os.IsNotExist(err) {
		t.Fatal("old executable should be cleaned up")
	}
}

func TestInstallRefusesBadOrMissingChecksum(t *testing.T) {
	name := updater.AssetName("9.9.9", "linux", "amd64")
	archive := tarGz(t, "bundleradar", "NEW-BINARY")

	for label, tc := range map[string]struct {
		checksum string
		sums     bool
		wantErr  string
	}{
		"mismatch":     {checksum: strings.Repeat("0", 64), sums: true, wantErr: "checksum mismatch"},
		"no checksums": {checksum: sum(archive), sums: false, wantErr: "no checksums.txt"},
	} {
		t.Run(label, func(t *testing.T) {
			f := newFakeRelease(t, "v9.9.9", name, archive, tc.checksum, tc.sums)
			c := client(f)
			rel, err := c.Release(context.Background(), "")
			if err != nil {
				t.Fatal(err)
			}
			target := writeTarget(t, "OLD-BINARY")
			err = c.Install(context.Background(), rel, target, "linux", "amd64")
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want %q", err, tc.wantErr)
			}
			if got, _ := os.ReadFile(target); string(got) != "OLD-BINARY" {
				t.Fatalf("binary was modified despite failure: %q", got)
			}
		})
	}
}

func TestInstallReportsUnsupportedPlatform(t *testing.T) {
	name := updater.AssetName("9.9.9", "linux", "amd64")
	archive := tarGz(t, "bundleradar", "x")
	f := newFakeRelease(t, "v9.9.9", name, archive, sum(archive), true)
	c := client(f)
	rel, _ := c.Release(context.Background(), "")
	err := c.Install(context.Background(), rel, writeTarget(t, "OLD"), "plan9", "mips")
	if err == nil || !strings.Contains(err.Error(), "no build for plan9/mips") {
		t.Fatalf("err = %v", err)
	}
}

func TestReleaseRejectsMalformedVersion(t *testing.T) {
	c := updater.New("1.0.0")
	if _, err := c.Release(context.Background(), "../../etc/passwd"); err == nil {
		t.Fatal("expected invalid version error")
	}
}

func TestNotifierChecksDailyAndNotifiesWeekly(t *testing.T) {
	name := updater.AssetName("9.9.9", "linux", "amd64")
	f := newFakeRelease(t, "v9.9.9", name, []byte("x"), "x", true)
	now := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
	n := &updater.Notifier{
		Client:    client(f),
		Current:   "1.0.0",
		CacheFile: filepath.Join(t.TempDir(), "cache", "update-check.json"),
		Now:       func() time.Time { return now },
	}

	msg := n.Check(context.Background())
	if !strings.Contains(msg, "1.0.0 → 9.9.9") || !strings.Contains(msg, "bundleradar update") {
		t.Fatalf("notice = %q", msg)
	}
	if got := n.Check(context.Background()); got != "" {
		t.Fatalf("second check should be silent, got %q", got)
	}
	if hits := f.hits.Load(); hits != 1 {
		t.Fatalf("network hits = %d, want 1 (cache should serve the second check)", hits)
	}

	now = now.Add(25 * time.Hour)
	if got := n.Check(context.Background()); got != "" {
		t.Fatalf("notice repeated within 7 days: %q", got)
	}
	if hits := f.hits.Load(); hits != 2 {
		t.Fatalf("network hits = %d, want 2 (daily refresh still runs)", hits)
	}

	now = now.Add(7 * 24 * time.Hour)
	if got := n.Check(context.Background()); got == "" {
		t.Fatal("expected the notice again after 7 days")
	}
}

func TestNotifierSilentWhenUpToDateOrOffline(t *testing.T) {
	name := updater.AssetName("1.0.0", "linux", "amd64")
	f := newFakeRelease(t, "v1.0.0", name, []byte("x"), "x", true)
	now := time.Now()
	cache := filepath.Join(t.TempDir(), "update-check.json")
	n := &updater.Notifier{Client: client(f), Current: "1.0.0", CacheFile: cache, Now: func() time.Time { return now }}
	if got := n.Check(context.Background()); got != "" {
		t.Fatalf("up to date should be silent, got %q", got)
	}

	f.server.Close()
	now = now.Add(48 * time.Hour)
	if got := n.Check(context.Background()); got != "" {
		t.Fatalf("offline check should be silent, got %q", got)
	}
}
