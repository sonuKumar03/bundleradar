package cmd_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sonuKumar03/bundleradar/cmd"
)

func releaseServer(t *testing.T, tag string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"tag_name":%q,"assets":[]}`, tag)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("BUNDLERADAR_UPDATE_API_URL", srv.URL+"/releases/latest")
}

func runUpdate(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := cmd.Execute(append([]string{"update"}, args...), &stdout, &stderr)
	return stdout.String(), stderr.String(), code
}

func TestUpdateCheckReportsAvailableVersion(t *testing.T) {
	releaseServer(t, "v99.0.0")
	out, _, code := runUpdate(t, "--check")
	if code != 0 || !strings.Contains(out, "99.0.0 is available") {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestUpdateCheckReportsUpToDate(t *testing.T) {
	releaseServer(t, "v0.0.1")
	out, _, code := runUpdate(t, "--check")
	if code != 0 || !strings.Contains(out, "up to date") {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestUpdateWithoutNewerVersionDoesNothing(t *testing.T) {
	releaseServer(t, "v0.0.1")
	out, _, code := runUpdate(t)
	if code != 0 || !strings.Contains(out, "already up to date") {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestUpdateRejectsMalformedVersionAsUsageError(t *testing.T) {
	releaseServer(t, "v99.0.0")
	_, _, code := runUpdate(t, "--version", "not-a-version")
	if code != cmd.ExitCodeUsage {
		t.Fatalf("code = %d, want usage error", code)
	}
}
