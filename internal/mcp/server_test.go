package mcp_test

import (
	"context"
	"os"
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

func TestHandleGateLoadsConfigAndAllowsExplicitOverride(t *testing.T) {
	statsPath, err := filepath.Abs("../../testdata/minimal/stats.json")
	if err != nil {
		t.Fatal(err)
	}
	configDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(configDir, ".bundleradar.yml"), []byte("budgets:\n  initial_js_max: 1B\n"), 0600); err != nil {
		t.Fatal(err)
	}
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(configDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldDir) })

	tool := mcp.NewServer().GetTool("bundle_gate")
	call := func(args map[string]any) string {
		t.Helper()
		res, err := tool.Handler(context.Background(), mcpspec.CallToolRequest{
			Params: mcpspec.CallToolParams{Name: "bundle_gate", Arguments: args},
		})
		if err != nil || res == nil {
			t.Fatalf("call tool: result=%+v error=%v", res, err)
		}
		if res.IsError {
			t.Fatalf("invalid gate response: %+v", res)
		}
		return res.Content[0].(mcpspec.TextContent).Text
	}

	result := call(map[string]any{"path": statsPath})
	if !strings.Contains(result, `"passed": false`) || !strings.Contains(result, `"rule": "MAX_INITIAL_SIZE"`) {
		t.Fatalf("config-only gate response = %s, want MAX_INITIAL_SIZE failure", result)
	}
	if result := call(map[string]any{"path": statsPath, "max_initial": "5MB"}); !strings.Contains(result, `"passed": true`) {
		t.Fatal("explicit max_initial should override the config limit")
	}
}
