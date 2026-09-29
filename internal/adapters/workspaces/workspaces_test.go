package workspaces_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sonuKumar03/bundleradar/internal/adapters/parsers"
	"github.com/sonuKumar03/bundleradar/internal/adapters/workspaces"
)

func TestExplicitResolver(t *testing.T) {
	resolver := &workspaces.ExplicitResolver{
		Specs: []string{"portal=dist/portal/stats.json:dist/portal/browser", "admin=dist/admin/stats.json"},
	}

	targets, err := resolver.Resolve(context.Background(), ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(targets))
	}

	if targets[0].Name != "portal" || targets[0].StatsPath != "dist/portal/stats.json" || targets[0].DistPath != "dist/portal/browser" {
		t.Fatalf("unexpected target 0: %+v", targets[0])
	}
	if targets[1].Name != "admin" || targets[1].StatsPath != "dist/admin/stats.json" {
		t.Fatalf("unexpected target 1: %+v", targets[1])
	}
}

func TestMonorepoResolver(t *testing.T) {
	tmpDir := t.TempDir()

	// Mock a pnpm workspace
	pnpmYaml := `packages:
  - 'apps/*'
`
	if err := os.WriteFile(filepath.Join(tmpDir, "pnpm-workspace.yaml"), []byte(pnpmYaml), 0644); err != nil {
		t.Fatalf("failed to write pnpm-workspace.yaml: %v", err)
	}

	// Create apps/web with build output
	webDir := filepath.Join(tmpDir, "apps", "web")
	_ = os.MkdirAll(filepath.Join(webDir, "dist"), 0755)
	_ = os.WriteFile(filepath.Join(webDir, "dist", "stats.json"), []byte("{}"), 0644)
	_ = os.WriteFile(filepath.Join(webDir, "package.json"), []byte(`{"name": "@myorg/web"}`), 0644)

	reg := workspaces.DefaultRegistry()
	targets, err := reg.Resolve(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("unexpected error resolving monorepo: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("expected 1 target discovered, got %d", len(targets))
	}

	if targets[0].Name != "web" && targets[0].Name != "@myorg/web" {
		t.Fatalf("unexpected discovered target name: %q", targets[0].Name)
	}
}

func TestNxResolver(t *testing.T) {
	nxDir := filepath.Join("..", "..", "..", "testdata", "nx-workspace")
	resolver := &workspaces.NxResolver{}

	if !resolver.Detect(nxDir) {
		t.Fatalf("expected nx resolver to detect nx-workspace")
	}

	targets, err := resolver.Resolve(context.Background(), nxDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(targets) < 2 {
		t.Fatalf("expected at least 2 targets from nx-workspace, got %d", len(targets))
	}

	targetNames := make(map[string]bool)
	for _, target := range targets {
		targetNames[target.Name] = true
	}

	expected := []string{"admin-dashboard", "portal"}
	for _, exp := range expected {
		if !targetNames[exp] {
			t.Errorf("expected target %q not found in nx-workspace", exp)
		}
	}
}

func TestRegistryExplainsAngularCLIWorkspacesNeedExplicitTargets(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "angular.json"), []byte(`{"projects":{}}`), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := workspaces.DefaultRegistry().Resolve(context.Background(), root)
	if err == nil || !strings.Contains(err.Error(), "--app NAME=STATS[:DIST]") {
		t.Fatalf("expected explicit target guidance for Angular CLI workspace, got %v", err)
	}
}

func TestNxResolverUsesProjectNameAndBuildOutputPath(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "nx.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	projectDir := filepath.Join(root, "apps", "web")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}
	project := `{"name":"storefront","projectType":"application","targets":{"build":{"options":{"outputPath":"out/storefront"}}}}`
	if err := os.WriteFile(filepath.Join(projectDir, "project.json"), []byte(project), 0644); err != nil {
		t.Fatal(err)
	}
	statsPath := filepath.Join(root, "out", "storefront", "stats.json")
	if err := os.MkdirAll(filepath.Dir(statsPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statsPath, []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	targets, err := (&workspaces.NxResolver{}).Resolve(context.Background(), root)
	if err != nil {
		t.Fatalf("resolve Nx projects: %v", err)
	}
	if len(targets) != 1 || targets[0].Name != "storefront" || targets[0].StatsPath != statsPath {
		t.Fatalf("expected storefront at %q, got %+v", statsPath, targets)
	}
}

func TestNxResolver_EndToEndAngularScan(t *testing.T) {
	nxDir := filepath.Join("..", "..", "..", "testdata", "nx-workspace")
	resolver := &workspaces.NxResolver{}
	reg := parsers.DefaultRegistry()

	targets, err := resolver.Resolve(context.Background(), nxDir)
	if err != nil {
		t.Fatalf("unexpected error resolving targets: %v", err)
	}

	for _, target := range targets {
		if _, err := os.Stat(target.StatsPath); os.IsNotExist(err) {
			t.Skipf("skipping target %s: stats not found at %s", target.Name, target.StatsPath)
		}

		p, err := reg.Resolve(target)
		if err != nil {
			t.Fatalf("[%s] failed to resolve parser: %v", target.Name, err)
		}

		if p.Name() != "angular" {
			t.Fatalf("[%s] expected parser 'angular', got %q", target.Name, p.Name())
		}

		bundle, err := p.Parse(context.Background(), target)
		if err != nil {
			t.Fatalf("[%s] failed to parse bundle: %v", target.Name, err)
		}

		ep, ok := bundle.Entrypoints["main"]
		if !ok {
			t.Fatalf("[%s] expected main entrypoint in bundle", target.Name)
		}

		t.Logf("[%s] Initial Bytes: %d, Async Bytes: %d, Chunks: %d",
			target.Name, ep.InitialBytes, ep.AsyncBytes, len(bundle.Chunks))

		switch target.Name {
		case "admin-dashboard":
			if ep.InitialBytes != 1151927 {
				t.Errorf("admin-dashboard initial bytes mismatch: got %d, want 1151927", ep.InitialBytes)
			}
		case "portal":
			if ep.InitialBytes != 2186272 {
				t.Errorf("portal initial bytes mismatch: got %d, want 2186272", ep.InitialBytes)
			}
			if ep.AsyncBytes != 88854 {
				t.Errorf("portal async bytes mismatch: got %d, want 88854", ep.AsyncBytes)
			}
		}
	}
}
