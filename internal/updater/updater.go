// Package updater implements self-update for the bundleradar binary: it looks
// up GitHub releases, downloads the archive for the running platform, verifies
// it against the release's checksums.txt, and atomically replaces the binary.
package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultAPIURL is the GitHub API endpoint for the latest release.
	DefaultAPIURL = "https://api.github.com/repos/sonuKumar03/bundleradar/releases/latest"
	// APIURLEnv overrides DefaultAPIURL (release mirrors and tests).
	APIURLEnv = "BUNDLERADAR_UPDATE_API_URL"

	maxDownloadBytes = 256 << 20
)

var tagPattern = regexp.MustCompile(`^v?[0-9]+\.[0-9]+\.[0-9]+$`)

// Asset is a downloadable release file.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

// Release is the subset of a GitHub release the updater needs.
type Release struct {
	Tag    string  `json:"tag_name"`
	Assets []Asset `json:"assets"`
}

// Version returns the release version without the leading "v".
func (r *Release) Version() string { return strings.TrimPrefix(r.Tag, "v") }

// Client talks to the GitHub releases API.
type Client struct {
	HTTP      *http.Client
	APIURL    string
	UserAgent string
}

// New returns a Client configured from the environment.
func New(currentVersion string) *Client {
	apiURL := os.Getenv(APIURLEnv)
	if apiURL == "" {
		apiURL = DefaultAPIURL
	}
	return &Client{
		HTTP:      &http.Client{Timeout: 5 * time.Minute},
		APIURL:    apiURL,
		UserAgent: "bundleradar/" + currentVersion,
	}
}

// Release fetches the latest release, or the release for tag when non-empty.
func (c *Client) Release(ctx context.Context, tag string) (*Release, error) {
	url := c.APIURL
	if tag != "" {
		if !tagPattern.MatchString(tag) {
			return nil, fmt.Errorf("invalid version %q (want e.g. v2.5.1)", tag)
		}
		if !strings.HasPrefix(tag, "v") {
			tag = "v" + tag
		}
		url = strings.TrimSuffix(url, "/latest") + "/tags/" + tag
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", c.UserAgent)
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("look up release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound && tag != "" {
		return nil, fmt.Errorf("release %s not found", tag)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("look up release: unexpected status %s", resp.Status)
	}
	var rel Release
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&rel); err != nil {
		return nil, fmt.Errorf("decode release: %w", err)
	}
	if rel.Tag == "" {
		return nil, errors.New("release has no tag")
	}
	return &rel, nil
}

// ParseVersion parses MAJOR.MINOR.PATCH, ignoring a leading "v" and any
// pre-release or build suffix.
func ParseVersion(s string) ([3]int, bool) {
	var v [3]int
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return v, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return v, false
		}
		v[i] = n
	}
	return v, true
}

// Newer reports whether latest is a strictly newer version than current.
// It is false when either version cannot be parsed.
func Newer(latest, current string) bool {
	l, ok1 := ParseVersion(latest)
	c, ok2 := ParseVersion(current)
	if !ok1 || !ok2 {
		return false
	}
	for i := range l {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}

// AssetName is the release archive name for a version and platform; it must
// match .goreleaser.yml and install.sh.
func AssetName(version, goos, goarch string) string {
	ext := "tar.gz"
	if goos == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("bundleradar_%s_%s_%s.%s", strings.TrimPrefix(version, "v"), goos, goarch, ext)
}

// BinaryName is the executable file name inside the archive.
func BinaryName(goos string) string {
	if goos == "windows" {
		return "bundleradar.exe"
	}
	return "bundleradar"
}

// ExecutablePath returns the resolved path of the running binary.
func ExecutablePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

// Install downloads rel for the given platform, verifies its checksum, and
// replaces the binary at targetPath. targetPath is left untouched on failure.
func (c *Client) Install(ctx context.Context, rel *Release, targetPath, goos, goarch string) error {
	archiveName := AssetName(rel.Version(), goos, goarch)
	var archiveURL, sumsURL string
	for _, a := range rel.Assets {
		switch a.Name {
		case archiveName:
			archiveURL = a.URL
		case "checksums.txt":
			sumsURL = a.URL
		}
	}
	if archiveURL == "" {
		return fmt.Errorf("release %s has no build for %s/%s", rel.Tag, goos, goarch)
	}
	if sumsURL == "" {
		return fmt.Errorf("release %s has no checksums.txt; refusing to install an unverified binary", rel.Tag)
	}

	sums, err := c.download(ctx, sumsURL)
	if err != nil {
		return fmt.Errorf("download checksums: %w", err)
	}
	want, err := checksumFor(sums, archiveName)
	if err != nil {
		return err
	}
	archive, err := c.download(ctx, archiveURL)
	if err != nil {
		return fmt.Errorf("download %s: %w", archiveName, err)
	}
	got := sha256.Sum256(archive)
	if hex.EncodeToString(got[:]) != want {
		return fmt.Errorf("checksum mismatch for %s; refusing to install", archiveName)
	}

	binary, err := extractBinary(archive, archiveName, BinaryName(goos))
	if err != nil {
		return err
	}
	return replaceBinary(targetPath, binary, goos)
}

func (c *Client) download(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxDownloadBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxDownloadBytes {
		return nil, errors.New("download exceeds size limit")
	}
	return data, nil
}

func checksumFor(sums []byte, name string) (string, error) {
	for line := range strings.SplitSeq(string(sums), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("checksums.txt has no entry for %s", name)
}

func extractBinary(archive []byte, archiveName, binaryName string) ([]byte, error) {
	if strings.HasSuffix(archiveName, ".zip") {
		zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
		if err != nil {
			return nil, fmt.Errorf("open archive: %w", err)
		}
		for _, f := range zr.File {
			if path.Base(f.Name) != binaryName || f.FileInfo().IsDir() {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(io.LimitReader(rc, maxDownloadBytes))
		}
		return nil, fmt.Errorf("%s not found in archive", binaryName)
	}

	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("open archive: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("%s not found in archive", binaryName)
		}
		if err != nil {
			return nil, fmt.Errorf("read archive: %w", err)
		}
		if hdr.Typeflag == tar.TypeReg && path.Base(hdr.Name) == binaryName {
			return io.ReadAll(io.LimitReader(tr, maxDownloadBytes))
		}
	}
}

// replaceBinary writes data next to target and swaps it in. The new file is
// fully written and synced before it replaces the old binary.
func replaceBinary(target string, data []byte, goos string) error {
	dir := filepath.Dir(target)
	mode := os.FileMode(0o755)
	if fi, err := os.Stat(target); err == nil {
		mode = fi.Mode().Perm() | 0o111
	}

	tmp, err := os.CreateTemp(dir, ".bundleradar-update-*")
	if err != nil {
		return permissionHint(dir, err)
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Chmod(tmpPath, mode); err != nil {
		cleanup()
		return err
	}

	if goos == "windows" {
		// A running .exe cannot be overwritten but can be renamed.
		old := target + ".old"
		_ = os.Remove(old)
		if err := os.Rename(target, old); err != nil {
			cleanup()
			return permissionHint(dir, err)
		}
		if err := os.Rename(tmpPath, target); err != nil {
			_ = os.Rename(old, target)
			cleanup()
			return err
		}
		_ = os.Remove(old)
		return nil
	}

	if err := os.Rename(tmpPath, target); err != nil {
		cleanup()
		return permissionHint(dir, err)
	}
	return nil
}

func permissionHint(dir string, err error) error {
	if errors.Is(err, os.ErrPermission) {
		return fmt.Errorf("cannot write to %s (%w); re-run with sudo or reinstall with install.sh", dir, err)
	}
	return err
}

// CurrentPlatform returns the GOOS/GOARCH the running binary was built for.
func CurrentPlatform() (string, string) { return runtime.GOOS, runtime.GOARCH }
