package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	mcpspec "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/sonuKumar03/bundleradar/internal/adapters/workspaces"
	"github.com/sonuKumar03/bundleradar/internal/core"
	"github.com/sonuKumar03/bundleradar/internal/core/diff"
	"github.com/sonuKumar03/bundleradar/pkg/bundleradar"
)

// ServerVersion references bundleradar.ToolVersion as the single source of truth.
var ServerVersion = bundleradar.ToolVersion

// NewServer creates and initializes a bundleradar MCP server with all v2 tools and resources.
func NewServer() *server.MCPServer {
	s := server.NewMCPServer(
		"bundleradar",
		ServerVersion,
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(false, false),
		server.WithInstructions("BundleRadar MCP server provides universal bundle inspection, size budget validation, and regression diffing for AI coding agents."),
	)

	registerTools(s)
	registerResources(s)

	return s
}

func registerTools(s *server.MCPServer) {
	// 1. bundle_scan
	s.AddTool(mcpspec.NewTool(
		"bundle_scan",
		mcpspec.WithDescription("Analyze and summarize bundle sizes (initial JS, async JS, total assets) and top contributing npm packages."),
		mcpspec.WithReadOnlyHintAnnotation(true),
		mcpspec.WithDestructiveHintAnnotation(false),
		mcpspec.WithIdempotentHintAnnotation(true),
		mcpspec.WithOpenWorldHintAnnotation(false),
		mcpspec.WithString("path", mcpspec.Required(), mcpspec.Description("Path to stats.json, metafile.json, or manifest.json file.")),
		mcpspec.WithString("dist", mcpspec.Description("Optional path to emitted browser dist directory.")),
		mcpspec.WithString("bundler", mcpspec.Description("Optional bundler override: esbuild, angular, vite, webpack (auto-detected if omitted).")),
		mcpspec.WithInteger("top", mcpspec.Description("Max number of top packages to include in summary. Defaults to 10.")),
	), handleScan)

	// 2. bundle_diff
	s.AddTool(mcpspec.NewTool(
		"bundle_diff",
		mcpspec.WithDescription("Compare current build against a local baseline stats file or BundleRadar scan JSON file and calculate size deltas with regression attribution."),
		mcpspec.WithReadOnlyHintAnnotation(true),
		mcpspec.WithDestructiveHintAnnotation(false),
		mcpspec.WithIdempotentHintAnnotation(true),
		mcpspec.WithOpenWorldHintAnnotation(false),
		mcpspec.WithString("path", mcpspec.Required(), mcpspec.Description("Path to current build stats/metafile JSON.")),
		mcpspec.WithString("against", mcpspec.Required(), mcpspec.Description("Path to a baseline stats/metafile/manifest file or BundleRadar scan JSON file. Git refs are not supported by MCP tools.")),
		mcpspec.WithString("drift_threshold", mcpspec.Description("Byte threshold to bucket micro-drift (e.g. '1KB', '500B'). Defaults to '1KB'.")),
	), handleDiff)

	// 3. bundle_gate
	s.AddTool(mcpspec.NewTool(
		"bundle_gate",
		mcpspec.WithDescription("Validate bundle sizes, regressions, and architecture rules against policy budgets in CI."),
		mcpspec.WithReadOnlyHintAnnotation(true),
		mcpspec.WithDestructiveHintAnnotation(false),
		mcpspec.WithIdempotentHintAnnotation(true),
		mcpspec.WithOpenWorldHintAnnotation(false),
		mcpspec.WithString("path", mcpspec.Required(), mcpspec.Description("Path to stats/metafile JSON file.")),
		mcpspec.WithString("against", mcpspec.Description("Optional local baseline stats/metafile/manifest file or BundleRadar scan JSON file. Git refs are not supported by MCP tools.")),
		mcpspec.WithString("max_initial", mcpspec.Description("Maximum initial JS budget (e.g. '250KB', '1MB').")),
		mcpspec.WithString("max_lazy", mcpspec.Description("Maximum lazy JS budget (e.g. '500KB').")),
		mcpspec.WithString("max_total", mcpspec.Description("Maximum total JS budget (e.g. '1.5MB').")),
		mcpspec.WithString("max_initial_delta", mcpspec.Description("Maximum allowed increase vs baseline (e.g. '10KB', '0B').")),
		mcpspec.WithString("max_total_delta", mcpspec.Description("Maximum allowed total JS increase vs baseline.")),
		mcpspec.WithString("max_css", mcpspec.Description("Maximum total CSS budget (e.g. '100KB', '1MB').")),
		mcpspec.WithString("max_assets", mcpspec.Description("Maximum total asset budget for images, fonts and media (e.g. '2MB').")),
		mcpspec.WithArray("forbid", mcpspec.WithStringItems(), mcpspec.Description("List of package names forbidden from appearing in bundle.")),
	), handleGate)

	// 4. workspace_summary
	s.AddTool(mcpspec.NewTool(
		"workspace_summary",
		mcpspec.WithDescription("List discovered application targets and their stats/dist paths in a supported workspace. This does not scan bundle contents."),
		mcpspec.WithReadOnlyHintAnnotation(true),
		mcpspec.WithDestructiveHintAnnotation(false),
		mcpspec.WithIdempotentHintAnnotation(true),
		mcpspec.WithOpenWorldHintAnnotation(false),
		mcpspec.WithString("root", mcpspec.Description("Root directory of the workspace. Defaults to current directory.")),
	), handleWorkspaceSummary)
}

func registerResources(s *server.MCPServer) {
	s.AddResource(
		mcpspec.NewResource(
			"bundleradar://rules",
			"BundleRadar Optimization Rules",
			mcpspec.WithResourceDescription("Common rules and guidelines for reducing web bundle sizes."),
			mcpspec.WithMIMEType("application/json"),
		),
		func(ctx context.Context, req mcpspec.ReadResourceRequest) ([]mcpspec.ResourceContents, error) {
			rules := map[string]any{
				"rules": []map[string]string{
					{"rule": "DYNAMIC_IMPORTS", "advice": "Move heavy feature routes behind dynamic import() to reduce initial JavaScript."},
					{"rule": "MODERN_LIBS", "advice": "Replace legacy libraries like moment with date-fns or native Intl API."},
					{"rule": "DEDUPLICATION", "advice": "Ensure npm dependencies are not bundled in multiple conflicting versions."},
				},
			}
			data, _ := json.MarshalIndent(rules, "", "  ")
			return []mcpspec.ResourceContents{
				mcpspec.TextResourceContents{
					URI:      "bundleradar://rules",
					MIMEType: "application/json",
					Text:     string(data),
				},
			}, nil
		},
	)
}

func handleScan(ctx context.Context, req mcpspec.CallToolRequest) (*mcpspec.CallToolResult, error) {
	path := req.GetString("path", "")
	dist := req.GetString("dist", "")
	bundler := req.GetString("bundler", "")
	entry := req.GetString("entry", "")
	top := req.GetInt("top", 10)

	client := bundleradar.New()
	bundle, err := client.Scan(ctx, bundleradar.ScanOptions{
		StatsPath: path,
		DistPath:  dist,
		Bundler:   bundler,
	})
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("Failed to scan bundle: %v", err)), nil
	}
	if entry != "" {
		if bundle, err = bundle.ScopeToEntrypoint(entry); err != nil {
			return mcpspec.NewToolResultError(err.Error()), nil
		}
	}

	result := map[string]any{
		"entrypoints":  bundle.Entrypoints,
		"topPackages":  bundle.TopPackages(top),
		"totalInitial": bundle.TotalInitialBytes(),
		"totalAsync":   bundle.TotalAsyncBytes(),
		"chunkCount":   len(bundle.Chunks),
	}
	if len(bundle.Warnings) > 0 {
		result["warnings"] = bundle.Warnings
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("JSON marshal error: %v", err)), nil
	}
	return mcpspec.NewToolResultText(string(data)), nil
}

func handleDiff(ctx context.Context, req mcpspec.CallToolRequest) (*mcpspec.CallToolResult, error) {
	path := req.GetString("path", "")
	against := req.GetString("against", "")
	driftThreshold := req.GetString("drift_threshold", "1KB")
	driftBytes, err := bundleradar.ParseBytes(driftThreshold)
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("Invalid drift_threshold: %v", err)), nil
	}

	dist := req.GetString("dist", "")
	bundler := req.GetString("bundler", "")
	entry := req.GetString("entry", "")

	client := bundleradar.New()
	currentBundle, err := client.Scan(ctx, bundleradar.ScanOptions{StatsPath: path, DistPath: dist, Bundler: bundler})
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("Failed to scan current bundle: %v", err)), nil
	}

	baseBundle, err := scanBaseline(ctx, client, against, bundler)
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("Failed to scan baseline bundle: %v", err)), nil
	}

	if entry != "" {
		if currentBundle, err = currentBundle.ScopeToEntrypoint(entry); err != nil {
			return mcpspec.NewToolResultError(err.Error()), nil
		}
		if baseBundle, err = baseBundle.ScopeToEntrypoint(entry); err != nil {
			return mcpspec.NewToolResultError(fmt.Sprintf("baseline: %v", err)), nil
		}
	}

	diffResult := client.Diff(baseBundle, currentBundle, diff.Options{
		DriftThreshold:   driftBytes,
		IncludeUnchanged: req.GetBool("include_unchanged", false),
	})

	data, err := json.MarshalIndent(diffResult, "", "  ")
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("JSON marshal error: %v", err)), nil
	}
	return mcpspec.NewToolResultText(string(data)), nil
}

func handleGate(ctx context.Context, req mcpspec.CallToolRequest) (*mcpspec.CallToolResult, error) {
	path := req.GetString("path", "")
	against := req.GetString("against", "")
	maxInitial := req.GetString("max_initial", "")
	maxLazy := req.GetString("max_lazy", "")
	maxTotal := req.GetString("max_total", "")
	maxInitialDelta := req.GetString("max_initial_delta", "")
	maxTotalDelta := req.GetString("max_total_delta", "")
	maxCSS := req.GetString("max_css", "")
	maxAssets := req.GetString("max_assets", "")
	forbidList := req.GetStringSlice("forbid", nil)
	if (maxInitialDelta != "" || maxTotalDelta != "") && against == "" {
		return mcpspec.NewToolResultError("delta budgets require a readable baseline in against"), nil
	}

	cfg, cfgPath, err := loadConfigFor(path)
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("Failed to load config: %v", err)), nil
	}
	dist := req.GetString("dist", "")
	bundler := req.GetString("bundler", "")
	entry := req.GetString("entry", "")
	if cfg != nil {
		if dist == "" && cfg.Scan.Dist != "" {
			dist = bundleradar.ResolvePath(cfgPath, cfg.Scan.Dist)
		}
		if bundler == "" {
			bundler = cfg.Scan.Bundler
		}
		if entry == "" {
			entry = cfg.Scan.Entry
		}
	}

	client := bundleradar.New()
	bundle, err := client.Scan(ctx, bundleradar.ScanOptions{StatsPath: path, DistPath: dist, Bundler: bundler})
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("Failed to scan bundle: %v", err)), nil
	}
	if entry != "" {
		if bundle, err = bundle.ScopeToEntrypoint(entry); err != nil {
			return mcpspec.NewToolResultError(err.Error()), nil
		}
	}

	var d *bundleradar.BundleDiff
	if against != "" {
		baseBundle, err := scanBaseline(ctx, client, against, bundler)
		if err != nil {
			return mcpspec.NewToolResultError(fmt.Sprintf("Failed to scan baseline bundle: %v", err)), nil
		}
		if entry != "" {
			if baseBundle, err = baseBundle.ScopeToEntrypoint(entry); err != nil {
				return mcpspec.NewToolResultError(fmt.Sprintf("baseline: %v", err)), nil
			}
		}
		d = client.Diff(baseBundle, bundle, diff.Options{})
	}

	pol, err := cfg.ToPolicy()
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("Invalid config: %v", err)), nil
	}
	if err := bundleradar.ApplyOverrides(&pol, bundleradar.PolicyOverrides{
		MaxInitial:      maxInitial,
		MaxLazy:         maxLazy,
		MaxTotal:        maxTotal,
		MaxInitialDelta: maxInitialDelta,
		MaxTotalDelta:   maxTotalDelta,
		MaxCSS:          maxCSS,
		MaxAssets:       maxAssets,
	}); err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("Invalid policy override: %v", err)), nil
	}
	if args, ok := req.Params.Arguments.(map[string]any); ok {
		if _, explicit := args["forbid"]; explicit {
			pol.ForbiddenPkgs = forbidList
		}
	}

	evalRes := client.Gate(bundle, d, pol)

	data, err := json.MarshalIndent(evalRes, "", "  ")
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("JSON marshal error: %v", err)), nil
	}
	return mcpspec.NewToolResultText(string(data)), nil
}

// loadConfigFor finds .bundleradar.yml next to (or above) the stats file, then
// falls back to the server's working directory.
func loadConfigFor(statsPath string) (*bundleradar.Config, string, error) {
	if abs, err := filepath.Abs(statsPath); err == nil {
		if cfg, p, err := bundleradar.FindAndLoadConfig(filepath.Dir(abs)); err != nil || cfg != nil {
			return cfg, p, err
		}
	}
	return bundleradar.FindAndLoadConfig("")
}

func scanBaseline(ctx context.Context, client *bundleradar.Client, path, bundler string) (*core.Bundle, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open baseline file %q (MCP accepts local files, not git refs): %w", path, err)
	}
	defer f.Close()

	header := make([]byte, 4096)
	n, _ := f.Read(header)
	if bytes.Contains(header[:n], []byte(`"metadata"`)) && bytes.Contains(header[:n], []byte(`"entrypoints"`)) {
		if _, err := f.Seek(0, 0); err != nil {
			return nil, err
		}
		var baseline core.Bundle
		if err := json.NewDecoder(f).Decode(&baseline); err == nil && baseline.Metadata.Bundler != "" && baseline.Entrypoints != nil {
			return &baseline, nil
		}
	}
	return client.Scan(ctx, bundleradar.ScanOptions{StatsPath: path, Bundler: bundler})
}

func handleWorkspaceSummary(ctx context.Context, req mcpspec.CallToolRequest) (*mcpspec.CallToolResult, error) {
	root := req.GetString("root", ".")
	targets, err := workspaces.DefaultRegistry().Resolve(ctx, root)
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("Failed to resolve workspace: %v", err)), nil
	}

	data, err := json.MarshalIndent(targets, "", "  ")
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("JSON marshal error: %v", err)), nil
	}
	return mcpspec.NewToolResultText(string(data)), nil
}
