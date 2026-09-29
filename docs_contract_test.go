package main_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"

	"github.com/sonuKumar03/bundleradar/cmd"
	"github.com/sonuKumar03/bundleradar/pkg/bundleradar"
)

// TestDocumentationContract_ActionInputs parses action.yml and verifies that all
// inputs referenced in README.md and docs/index.html
// correspond to real, declared inputs in action.yml.
func TestDocumentationContract_ActionInputs(t *testing.T) {
	actionData, err := os.ReadFile("action.yml")
	if err != nil {
		t.Fatalf("failed to read action.yml: %v", err)
	}

	var actionDef struct {
		Inputs map[string]any `yaml:"inputs"`
	}
	if err := yaml.Unmarshal(actionData, &actionDef); err != nil {
		t.Fatalf("failed to parse action.yml: %v", err)
	}

	if len(actionDef.Inputs) == 0 {
		t.Fatal("action.yml has no inputs defined")
	}

	docFiles := []string{
		"README.md",
		filepath.Join("docs", "index.html"),
	}

	// Regex to extract with: blocks inside bundleradar GitHub Action YAML examples
	withBlockRegex := regexp.MustCompile(`(?s)uses:\s*[^'\n]*bundleradar[^\n]*\n\s*with:\s*\n((?:\s{8,14}[a-zA-Z0-9_-]+:\s*[^\n]*\n)+)`)
	keyRegex := regexp.MustCompile(`^\s*([a-zA-Z0-9_-]+):`)

	for _, docFile := range docFiles {
		content, err := os.ReadFile(docFile)
		if err != nil {
			t.Fatalf("failed to read %s: %v", docFile, err)
		}

		matches := withBlockRegex.FindAllStringSubmatch(string(content), -1)
		if len(matches) == 0 {
			continue
		}

		for _, match := range matches {
			lines := strings.Split(match[1], "\n")
			for _, line := range lines {
				line = strings.TrimRight(line, " \r\t")
				if line == "" {
					continue
				}
				keyMatch := keyRegex.FindStringSubmatch(line)
				if len(keyMatch) < 2 {
					continue
				}
				key := keyMatch[1]
				if _, ok := actionDef.Inputs[key]; !ok {
					t.Errorf("%s documents non-existent action input %q under with:", docFile, key)
				}
			}
		}
	}
}

// TestDocumentationContract_ConfigurationExamples ensures that every .bundleradar.yml
// example in public documentation parses cleanly using the strict configuration loader.
func TestDocumentationContract_ConfigurationExamples(t *testing.T) {
	docFiles := []string{
		"README.md",
		filepath.Join("docs", "index.html"),
	}

	mdYamlRegex := regexp.MustCompile("(?s)```ya?ml\\s*\n(# \\.bundleradar\\.yml.*?)```")
	htmlYamlRegex := regexp.MustCompile(`(?s)<pre><code>(# \.bundleradar\.yml.*?)</code></pre>`)

	for _, docFile := range docFiles {
		content, err := os.ReadFile(docFile)
		if err != nil {
			t.Fatalf("failed to read %s: %v", docFile, err)
		}

		var snippets []string
		for _, match := range mdYamlRegex.FindAllStringSubmatch(string(content), -1) {
			snippets = append(snippets, match[1])
		}
		for _, match := range htmlYamlRegex.FindAllStringSubmatch(string(content), -1) {
			snippets = append(snippets, match[1])
		}

		for _, snippet := range snippets {
			var cfg bundleradar.Config
			decoder := yaml.NewDecoder(bytes.NewReader([]byte(snippet)))
			decoder.KnownFields(true)
			if err := decoder.Decode(&cfg); err != nil {
				t.Errorf("%s contains invalid or unsupported .bundleradar.yml snippet:\n%s\nError: %v", docFile, snippet, err)
			}
		}
	}
}

// TestDocumentationContract_VersionSync validates version alignment across repo artifacts.
func TestDocumentationContract_VersionSync(t *testing.T) {
	rawVersion, err := os.ReadFile("VERSION")
	if err != nil {
		t.Fatalf("failed to read VERSION: %v", err)
	}
	version := strings.TrimSpace(string(rawVersion))
	if version == "" {
		t.Fatal("VERSION file is empty")
	}

	if bundleradar.ToolVersion != version {
		t.Errorf("bundleradar.ToolVersion (%s) != VERSION (%s)", bundleradar.ToolVersion, version)
	}

	// go.mod module path
	modData, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatalf("failed to read go.mod: %v", err)
	}
	if !strings.Contains(string(modData), "module github.com/sonuKumar03/bundleradar") {
		t.Errorf("go.mod does not declare module github.com/sonuKumar03/bundleradar")
	}

	website, err := os.ReadFile(filepath.Join("docs", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	for label, pattern := range map[string]string{
		"softwareVersion": `"softwareVersion": "([^"]+)"`,
		"visible badge":   `data-version-badge>bundleradar v([^<]+)<`,
	} {
		match := regexp.MustCompile(pattern).FindSubmatch(website)
		if len(match) != 2 || string(match[1]) != version {
			t.Errorf("docs/index.html %s = %q, want %q", label, match, version)
		}
	}
}

func TestDocumentationContract_JSONExamplesMatchCLI(t *testing.T) {
	stats := filepath.Join("testdata", "minimal", "stats.json")
	var scanOut, errs bytes.Buffer
	if code := cmd.Execute([]string{"scan", stats, "-f", "json"}, &scanOut, &errs); code != cmd.ExitCodeSuccess {
		t.Fatalf("scan JSON: code=%d error=%s", code, errs.String())
	}
	var scan map[string]json.RawMessage
	if err := json.Unmarshal(scanOut.Bytes(), &scan); err != nil {
		t.Fatalf("decode scan JSON: %v", err)
	}
	for _, key := range []string{"metadata", "entrypoints", "chunks", "modules", "assets"} {
		if _, ok := scan[key]; !ok {
			t.Errorf("scan JSON missing %q", key)
		}
	}
	if _, ok := scan["schemaVersion"]; ok {
		t.Error("scan JSON unexpectedly contains schemaVersion")
	}

	var diffOut bytes.Buffer
	errs.Reset()
	if code := cmd.Execute([]string{"diff", stats, "--against", stats, "-f", "json"}, &diffOut, &errs); code != cmd.ExitCodeSuccess {
		t.Fatalf("diff JSON: code=%d error=%s", code, errs.String())
	}
	var diffReport struct {
		Summary struct {
			InitialDeltaBytes int64 `json:"initialDeltaBytes"`
			LazyDeltaBytes    int64 `json:"lazyDeltaBytes"`
			TotalDeltaBytes   int64 `json:"totalDeltaBytes"`
		} `json:"summary"`
		Entrypoints map[string]json.RawMessage `json:"entrypoints"`
		Packages    []json.RawMessage          `json:"packages"`
	}
	if err := json.Unmarshal(diffOut.Bytes(), &diffReport); err != nil {
		t.Fatalf("decode diff JSON: %v", err)
	}
	if diffReport.Entrypoints == nil || diffReport.Packages == nil {
		t.Fatalf("diff JSON missing documented sections: %s", diffOut.String())
	}

	for _, doc := range []string{"docs/agents.md", filepath.Join(".agents", "skills", "bundleradar", "references", "json-schema.md")} {
		content, err := os.ReadFile(doc)
		if err != nil {
			t.Fatal(err)
		}
		text := string(content)
		for _, field := range []string{"metadata", "entrypoints", "chunks", "modules", "assets", "initialDeltaBytes", "lazyDeltaBytes", "totalDeltaBytes"} {
			if !strings.Contains(text, field) {
				t.Errorf("%s does not document actual JSON field %q", doc, field)
			}
		}
		for _, absent := range []string{"schemaVersion", "toolVersion", "packageDeltas", "entrypointDeltas"} {
			if strings.Contains(text, absent) {
				t.Errorf("%s documents unsupported JSON field %q", doc, absent)
			}
		}
	}
}

func TestDocumentationContract_MarketingClaims(t *testing.T) {
	website, err := os.ReadFile(filepath.Join("docs", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, claim := range []string{
		"Zero-Dependency Go CLI",
		"SUB-MILLISECOND SPEED",
		"< 5ms Execution",
		"Go v1.23+",
		"For coding assistants without direct MCP connections",
	} {
		if bytes.Contains(website, []byte(claim)) {
			t.Errorf("docs/index.html contains unsupported claim %q", claim)
		}
	}

	for _, name := range []string{"README.md", filepath.Join("docs", "index.html")} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		lower := strings.ToLower(string(data))
		for _, term := range []string{"self-contained", "runtime dependencies"} {
			if !strings.Contains(lower, term) {
				t.Errorf("%s must describe %q", name, term)
			}
		}
	}
}

// TestDocumentationContract_CLICommandsAndFlags verifies that all commands and flags
// mentioned in public documentation exist in the Cobra command structure.
func TestDocumentationContract_CLICommandsAndFlags(t *testing.T) {
	root := cmd.NewRootCommand()
	subcommands := make(map[string]map[string]bool)

	for _, c := range root.Commands() {
		flags := make(map[string]bool)
		c.Flags().VisitAll(func(f *pflag.Flag) {
			flags["--"+f.Name] = true
			if f.Shorthand != "" {
				flags["-"+f.Shorthand] = true
			}
		})
		c.PersistentFlags().VisitAll(func(f *pflag.Flag) {
			flags["--"+f.Name] = true
			if f.Shorthand != "" {
				flags["-"+f.Shorthand] = true
			}
		})
		subcommands[c.Name()] = flags
	}

	// Common documented commands and their flags to assert against the CLI tree
	expectedSpecs := map[string][]string{
		"scan":      {"--dist", "-d", "--format", "-f", "--output", "-o", "--top", "--entry", "-e", "--why", "--bundler"},
		"diff":      {"--against", "--format", "-f", "--output", "-o", "--drift-threshold", "--bundler", "--build-cmd", "--no-build"},
		"gate":      {"--dist", "-d", "--format", "-f", "--output", "-o", "--against", "--max-initial", "--max-total", "--max-initial-delta", "--forbid", "--detect-duplicate-pkgs"},
		"workspace": {"--root", "--format", "-f", "--output", "-o"},
		"mcp":       {},
	}

	for cmdName, expectedFlags := range expectedSpecs {
		flagsMap, exists := subcommands[cmdName]
		if !exists {
			t.Errorf("command %q documented but not registered on root command", cmdName)
			continue
		}
		for _, flag := range expectedFlags {
			if !flagsMap[flag] {
				t.Errorf("command %q missing expected documented flag %q", cmdName, flag)
			}
		}
	}
}

// TestDocumentationContract_EntryDocumentation verifies exact --entry examples and the
// TotalJS whole-browser invariant across README.md, docs/index.html, and action.yml.
func TestDocumentationContract_EntryDocumentation(t *testing.T) {
	actionData, err := os.ReadFile("action.yml")
	if err != nil {
		t.Fatalf("failed to read action.yml: %v", err)
	}
	var actionDef struct {
		Inputs map[string]struct {
			Description string `yaml:"description"`
			Required    bool   `yaml:"required"`
			Default     string `yaml:"default"`
		} `yaml:"inputs"`
	}
	if err := yaml.Unmarshal(actionData, &actionDef); err != nil {
		t.Fatalf("failed to parse action.yml: %v", err)
	}
	entryInput, ok := actionDef.Inputs["entry"]
	if !ok {
		t.Errorf("action.yml missing input 'entry'")
	} else {
		if entryInput.Required {
			t.Errorf("action.yml input 'entry' should not be required")
		}
		if entryInput.Default != "" {
			t.Errorf("action.yml input 'entry' default should be empty string, got %q", entryInput.Default)
		}
		if !strings.Contains(strings.ToLower(entryInput.Description), "entrypoint") {
			t.Errorf("action.yml input 'entry' description should mention 'entrypoint', got %q", entryInput.Description)
		}
	}

	docFiles := []string{
		"README.md",
		filepath.Join("docs", "index.html"),
	}

	for _, docFile := range docFiles {
		contentBytes, err := os.ReadFile(docFile)
		if err != nil {
			t.Fatalf("failed to read %s: %v", docFile, err)
		}
		content := string(contentBytes)

		// 1. Must document --entry and -e
		if !strings.Contains(content, "--entry") {
			t.Errorf("%s missing '--entry' documentation", docFile)
		}
		if !strings.Contains(content, "-e") {
			t.Errorf("%s missing '-e' documentation", docFile)
		}

		// 2. Must document GitHub Action 'entry' input
		if !strings.Contains(content, "entry") || (!strings.Contains(content, "entry:") && !strings.Contains(content, "`entry`")) {
			t.Errorf("%s missing GitHub Action 'entry' input documentation", docFile)
		}

		// 3. Document exact source/chunk matches and explain that globs are unsupported.
		lower := strings.ToLower(content)
		if !strings.Contains(content, "src/main.ts") {
			t.Errorf("%s missing source path entry example (e.g. src/main.ts)", docFile)
		}
		if !strings.Contains(content, "main.js") || !strings.Contains(lower, "glob") || !strings.Contains(lower, "not supported") {
			t.Errorf("%s must show an exact chunk name and say glob matching is unsupported", docFile)
		}

		// 4. Invariant: --entry scopes available entry data, while TotalJS reflects the whole browser build.
		hasTotalInvariant := (strings.Contains(lower, "totaljs") || strings.Contains(lower, "total js")) &&
			(strings.Contains(lower, "whole") || strings.Contains(lower, "entire") || strings.Contains(lower, "all"))
		hasReachability := strings.Contains(lower, "reachab") || strings.Contains(lower, "initial") || strings.Contains(lower, "trace")
		if !hasTotalInvariant || !hasReachability {
			t.Errorf("%s missing explanation of invariant: --entry scopes initial/lazy reachability while TotalJS reflects whole browser build", docFile)
		}
	}
}

func TestDocumentationContract_AgentPositioning(t *testing.T) {
	website, err := os.ReadFile(filepath.Join("docs", "index.html"))
	if err != nil {
		t.Fatal(err)
	}

	lower := strings.ToLower(string(website))
	for _, term := range []string{
		"mcp server",
		"agent skill",
		`href="#cmd-mcp"`,
		`href="#agent-skill"`,
		`id="cmd-mcp"`,
		`id="agent-skill"`,
	} {
		if !strings.Contains(lower, term) {
			t.Errorf("docs/index.html must contain %q", term)
		}
	}

	for _, metadata := range []struct {
		start string
		end   string
	}{
		{"<title>", "</title>"},
		{`<meta name="description"`, ">"},
		{`<meta property="og:title"`, ">"},
		{`<meta property="og:description"`, ">"},
	} {
		start := strings.Index(lower, metadata.start)
		if start < 0 {
			t.Errorf("docs/index.html is missing %q", metadata.start)
			continue
		}
		content := lower[start+len(metadata.start):]
		end := strings.Index(content, metadata.end)
		if end < 0 {
			t.Errorf("docs/index.html has malformed %q", metadata.start)
			continue
		}
		declaration := content[:end]
		if !strings.Contains(declaration, "mcp") || !strings.Contains(declaration, "agent skill") {
			t.Errorf("%s must name MCP and Agent Skill", declaration)
		}
	}
}

// TestDocumentationContract_ArtifactBaselineInputs ensures action.yml defines
// the persistent artifact baseline inputs with secure and non-breaking defaults.
func TestDocumentationContract_ArtifactBaselineInputs(t *testing.T) {
	actionData, err := os.ReadFile("action.yml")
	if err != nil {
		t.Fatalf("failed to read action.yml: %v", err)
	}

	var actionDef struct {
		Inputs map[string]struct {
			Description string `yaml:"description"`
			Required    bool   `yaml:"required"`
			Default     string `yaml:"default"`
		} `yaml:"inputs"`
	}
	if err := yaml.Unmarshal(actionData, &actionDef); err != nil {
		t.Fatalf("failed to parse action.yml: %v", err)
	}

	expectedInputs := map[string]struct {
		required    bool
		defaultVal  string
		descKeyword string
	}{
		"artifact-baseline": {
			required:    false,
			defaultVal:  "false",
			descKeyword: "artifact",
		},
		"artifact-name": {
			required:    false,
			defaultVal:  "",
			descKeyword: "artifact",
		},
		"upload-artifact-baseline": {
			required:    false,
			defaultVal:  "false",
			descKeyword: "baseline",
		},
		"github-token": {
			required:    false,
			defaultVal:  "${{ github.token }}",
			descKeyword: "token",
		},
	}

	for inputName, expected := range expectedInputs {
		input, ok := actionDef.Inputs[inputName]
		if !ok {
			t.Errorf("action.yml missing declared input %q", inputName)
			continue
		}
		if input.Required != expected.required {
			t.Errorf("action.yml input %q required=%v, expected %v", inputName, input.Required, expected.required)
		}
		if input.Default != expected.defaultVal {
			t.Errorf("action.yml input %q default=%q, expected %q", inputName, input.Default, expected.defaultVal)
		}
		if !strings.Contains(strings.ToLower(input.Description), expected.descKeyword) {
			t.Errorf("action.yml input %q description %q missing keyword %q", inputName, input.Description, expected.descKeyword)
		}
	}
}
