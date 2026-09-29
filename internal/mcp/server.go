package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	mcpspec "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/sonuKumar03/bundleradar/internal/adapters/workspaces"
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
		mcpspec.WithDescription("Compare current build against a baseline file or git ref and calculate size deltas with regression attribution."),
		mcpspec.WithReadOnlyHintAnnotation(true),
		mcpspec.WithDestructiveHintAnnotation(false),
		mcpspec.WithIdempotentHintAnnotation(true),
		mcpspec.WithOpenWorldHintAnnotation(false),
		mcpspec.WithString("path", mcpspec.Required(), mcpspec.Description("Path to current build stats/metafile JSON.")),
		mcpspec.WithString("against", mcpspec.Required(), mcpspec.Description("Path to baseline stats/metafile JSON or git ref (e.g. main, HEAD~1).")),
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
		mcpspec.WithString("against", mcpspec.Description("Optional baseline stats file to check regression deltas.")),
		mcpspec.WithString("max_initial", mcpspec.Description("Maximum initial JS budget (e.g. '250KB', '1MB').")),
		mcpspec.WithString("max_lazy", mcpspec.Description("Maximum lazy JS budget (e.g. '500KB').")),
		mcpspec.WithString("max_total", mcpspec.Description("Maximum total JS budget (e.g. '1.5MB').")),
		mcpspec.WithString("max_initial_delta", mcpspec.Description("Maximum allowed increase vs baseline (e.g. '10KB', '0B').")),
		mcpspec.WithString("max_total_delta", mcpspec.Description("Maximum allowed total JS increase vs baseline.")),
		mcpspec.WithArray("forbid", mcpspec.WithStringItems(), mcpspec.Description("List of package names forbidden from appearing in bundle.")),
	), handleGate)

	// 4. workspace_summary
	s.AddTool(mcpspec.NewTool(
		"workspace_summary",
		mcpspec.WithDescription("Discover and summarize all application targets across a monorepo workspace (Nx, pnpm, npm, yarn)."),
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

	result := map[string]any{
		"entrypoints": bundle.Entrypoints,
		"topPackages": bundle.TopPackages(top),
		"totalInitial": bundle.TotalInitialBytes(),
		"totalAsync":   bundle.TotalAsyncBytes(),
		"chunkCount":   len(bundle.Chunks),
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

	client := bundleradar.New()
	currentBundle, err := client.Scan(ctx, bundleradar.ScanOptions{StatsPath: path})
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("Failed to scan current bundle: %v", err)), nil
	}

	baseBundle, err := client.Scan(ctx, bundleradar.ScanOptions{StatsPath: against})
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("Failed to scan baseline bundle: %v", err)), nil
	}

	driftBytes, _ := bundleradar.ParseBytes(driftThreshold)
	diffResult := client.Diff(baseBundle, currentBundle, diff.Options{DriftThreshold: driftBytes})

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
	forbidList := req.GetStringSlice("forbid", nil)
	if (maxInitialDelta != "" || maxTotalDelta != "") && against == "" {
		return mcpspec.NewToolResultError("delta budgets require a readable baseline in against"), nil
	}

	client := bundleradar.New()
	bundle, err := client.Scan(ctx, bundleradar.ScanOptions{StatsPath: path})
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("Failed to scan bundle: %v", err)), nil
	}

	var d *bundleradar.BundleDiff
	if against != "" {
		baseBundle, err := client.Scan(ctx, bundleradar.ScanOptions{StatsPath: against})
		if err != nil {
			return mcpspec.NewToolResultError(fmt.Sprintf("Failed to scan baseline bundle: %v", err)), nil
		}
		d = client.Diff(baseBundle, bundle, diff.Options{})
	}

	cfg, _, err := bundleradar.FindAndLoadConfig("")
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("Failed to load config: %v", err)), nil
	}
	pol, err := cfg.ToPolicy()
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("Invalid config: %v", err)), nil
	}
	if maxInitial != "" {
		val, err := bundleradar.ParseBytes(maxInitial)
		if err != nil {
			return mcpspec.NewToolResultError(fmt.Sprintf("Invalid max_initial: %v", err)), nil
		}
		pol.MaxInitial = &val
	}
	if maxTotal != "" {
		val, err := bundleradar.ParseBytes(maxTotal)
		if err != nil {
			return mcpspec.NewToolResultError(fmt.Sprintf("Invalid max_total: %v", err)), nil
		}
		pol.MaxTotal = &val
	}
	if maxLazy != "" {
		val, err := bundleradar.ParseBytes(maxLazy)
		if err != nil {
			return mcpspec.NewToolResultError(fmt.Sprintf("Invalid max_lazy: %v", err)), nil
		}
		pol.MaxLazy = &val
	}
	if maxInitialDelta != "" {
		val, err := bundleradar.ParseBytes(maxInitialDelta)
		if err != nil {
			return mcpspec.NewToolResultError(fmt.Sprintf("Invalid max_initial_delta: %v", err)), nil
		}
		pol.MaxInitialDelta = &val
	}
	if maxTotalDelta != "" {
		val, err := bundleradar.ParseBytes(maxTotalDelta)
		if err != nil {
			return mcpspec.NewToolResultError(fmt.Sprintf("Invalid max_total_delta: %v", err)), nil
		}
		pol.MaxTotalDelta = &val
	}
	if args, ok := req.Params.Arguments.(map[string]any); ok {
		if _, explicit := args["forbid"]; explicit {
			pol.ForbiddenPkgs = forbidList
		}
	}
	pol.DetectDuplicatePkgs = true

	evalRes := client.Gate(bundle, d, pol)

	data, err := json.MarshalIndent(evalRes, "", "  ")
	if err != nil {
		return mcpspec.NewToolResultError(fmt.Sprintf("JSON marshal error: %v", err)), nil
	}
	return mcpspec.NewToolResultText(string(data)), nil
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
