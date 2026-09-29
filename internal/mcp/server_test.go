package mcp_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	mcpspec "github.com/mark3labs/mcp-go/mcp"
	"github.com/sonuKumar03/bundleradar/internal/mcp"
)

func TestNewServer_Metadata(t *testing.T) {
	s := mcp.NewServer()
	if s == nil {
		t.Fatal("expected non-nil server")
	}

	tools := s.ListTools()
	expectedTools := []string{
		"bundle_scan",
		"bundle_diff",
		"bundle_gate",
		"workspace_summary",
	}

	for _, name := range expectedTools {
		if _, ok := tools[name]; !ok {
			t.Errorf("missing expected tool: %s", name)
		}
	}

	resources := s.ListResources()
	if _, ok := resources["bundleradar://rules"]; !ok {
		t.Errorf("missing expected resource bundleradar://rules")
	}
}

func TestHandleScan(t *testing.T) {
	ctx := context.Background()
	s := mcp.NewServer()

	statsPath, _ := filepath.Abs("../../testdata/minimal/stats.json")
	distDir, _ := filepath.Abs("../../testdata/minimal/browser")

	req := mcpspec.CallToolRequest{
		Params: mcpspec.CallToolParams{
			Name: "bundle_scan",
			Arguments: map[string]any{
				"path": statsPath,
				"dist": distDir,
			},
		},
	}

	tool := s.GetTool("bundle_scan")
	if tool == nil {
		t.Fatal("bundle_scan tool not found")
	}

	res, err := tool.Handler(ctx, req)
	if err != nil {
		t.Fatalf("call tool error: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned error: %s", res.Content[0].(mcpspec.TextContent).Text)
	}

	text := res.Content[0].(mcpspec.TextContent).Text
	if !strings.Contains(text, "entrypoints") {
		t.Fatalf("expected entrypoints in output: %s", text)
	}
}

func TestHandleGateRejectsUnevaluatedBudgets(t *testing.T) {
	statsPath, _ := filepath.Abs("../../testdata/minimal/stats.json")
	cases := []struct {
		name string
		args map[string]any
	}{
		{"invalid budget", map[string]any{"path": statsPath, "max_initial": "bad"}},
		{"unreadable baseline", map[string]any{"path": statsPath, "against": "/missing/stats.json", "max_initial_delta": "0B"}},
		{"delta without baseline", map[string]any{"path": statsPath, "max_initial_delta": "0B"}},
	}
	tool := mcp.NewServer().GetTool("bundle_gate")
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := tool.Handler(context.Background(), mcpspec.CallToolRequest{
				Params: mcpspec.CallToolParams{Name: "bundle_gate", Arguments: tc.args},
			})
			if err != nil {
				t.Fatalf("call tool: %v", err)
			}
			if res == nil || !res.IsError {
				t.Fatalf("expected gate input error, got %+v", res)
			}
		})
	}
}
