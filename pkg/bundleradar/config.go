package bundleradar

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the .bundleradar.yml configuration schema.
type Config struct {
	Scan    ConfigScan    `yaml:"scan,omitempty"`
	Budgets ConfigBudgets `yaml:"budgets,omitempty"`
	Rules   ConfigRules   `yaml:"rules,omitempty"`
}

// ConfigScan holds default scan inputs so CI and agents don't need flags.
// Relative dist paths resolve against the directory of the config file.
type ConfigScan struct {
	Bundler string `yaml:"bundler,omitempty"`
	Dist    string `yaml:"dist,omitempty"`
	Entry   string `yaml:"entry,omitempty"`
}

// ConfigBudgets holds threshold limits for bundle metrics and deltas.
type ConfigBudgets struct {
	InitialJSMax    string `yaml:"initial_js_max,omitempty"`
	MaxLazy         string `yaml:"max_lazy,omitempty"`
	TotalMax        string `yaml:"total_max,omitempty"`
	MaxInitialDelta string `yaml:"max_initial_delta,omitempty"`
	MaxTotalDelta   string `yaml:"max_total_delta,omitempty"`
	MaxCSS          string `yaml:"max_css,omitempty"`
	MaxAssets       string `yaml:"max_assets,omitempty"`
}

// ConfigRules holds architectural lint rules for bundled packages.
type ConfigRules struct {
	DisallowPackages []string `yaml:"disallow_packages,omitempty"`
	// DetectDuplicatePackages defaults to true when omitted.
	DetectDuplicatePackages *bool `yaml:"detect_duplicate_packages,omitempty"`
}

// ResolvePath resolves a config-relative path against the config file's directory.
func ResolvePath(configPath, p string) string {
	if p == "" || filepath.IsAbs(p) || configPath == "" {
		return p
	}
	return filepath.Join(filepath.Dir(configPath), p)
}

// ToPolicy converts config budgets and rules to an evaluation Policy.
func (c *Config) ToPolicy() (Policy, error) {
	var pol Policy
	pol.DetectDuplicatePkgs = true
	if c == nil {
		return pol, nil
	}
	if c.Rules.DetectDuplicatePackages != nil {
		pol.DetectDuplicatePkgs = *c.Rules.DetectDuplicatePackages
	}

	if c.Budgets.MaxLazy != "" {
		val, err := ParseBytes(c.Budgets.MaxLazy)
		if err != nil {
			return pol, fmt.Errorf("config max_lazy: %w", err)
		}
		pol.MaxLazy = &val
	}

	if c.Budgets.MaxTotalDelta != "" {
		val, err := ParseBytes(c.Budgets.MaxTotalDelta)
		if err != nil {
			return pol, fmt.Errorf("config max_total_delta: %w", err)
		}
		pol.MaxTotalDelta = &val
	}

	if c.Budgets.InitialJSMax != "" {
		val, err := ParseBytes(c.Budgets.InitialJSMax)
		if err != nil {
			return pol, fmt.Errorf("config initial_js_max: %w", err)
		}
		pol.MaxInitial = &val
	}

	if c.Budgets.TotalMax != "" {
		val, err := ParseBytes(c.Budgets.TotalMax)
		if err != nil {
			return pol, fmt.Errorf("config total_max: %w", err)
		}
		pol.MaxTotal = &val
	}

	if c.Budgets.MaxInitialDelta != "" {
		val, err := ParseBytes(c.Budgets.MaxInitialDelta)
		if err != nil {
			return pol, fmt.Errorf("config max_initial_delta: %w", err)
		}
		pol.MaxInitialDelta = &val
	}

	if c.Budgets.MaxCSS != "" {
		val, err := ParseBytes(c.Budgets.MaxCSS)
		if err != nil {
			return pol, fmt.Errorf("config max_css: %w", err)
		}
		pol.MaxCSS = &val
	}

	if c.Budgets.MaxAssets != "" {
		val, err := ParseBytes(c.Budgets.MaxAssets)
		if err != nil {
			return pol, fmt.Errorf("config max_assets: %w", err)
		}
		pol.MaxAssets = &val
	}

	for _, p := range c.Rules.DisallowPackages {
		name := strings.TrimSpace(p)
		if name != "" {
			pol.ForbiddenPkgs = append(pol.ForbiddenPkgs, name)
		}
	}

	return pol, nil
}

// PolicyOverrides holds optional CLI/MCP flag values that override a
// config-derived Policy. Empty strings are ignored.
type PolicyOverrides struct {
	MaxInitial      string
	MaxLazy         string
	MaxTotal        string
	MaxInitialDelta string
	MaxTotalDelta   string
	MaxCSS          string
	MaxAssets       string
}

// ApplyOverrides merges non-empty flag values into an existing Policy.
func ApplyOverrides(pol *Policy, o PolicyOverrides) error {
	fields := []struct {
		flag string
		dest **int64
		name string
	}{
		{o.MaxInitial, &pol.MaxInitial, "max-initial"},
		{o.MaxLazy, &pol.MaxLazy, "max-lazy"},
		{o.MaxTotal, &pol.MaxTotal, "max-total"},
		{o.MaxInitialDelta, &pol.MaxInitialDelta, "max-initial-delta"},
		{o.MaxTotalDelta, &pol.MaxTotalDelta, "max-total-delta"},
		{o.MaxCSS, &pol.MaxCSS, "max-css"},
		{o.MaxAssets, &pol.MaxAssets, "max-assets"},
	}
	for _, f := range fields {
		if f.flag == "" {
			continue
		}
		val, err := ParseBytes(f.flag)
		if err != nil {
			return fmt.Errorf("invalid --%s: %w", f.name, err)
		}
		*f.dest = &val
	}
	return nil
}

// LoadConfig loads and strictly parses a .bundleradar.yml file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}

// FindAndLoadConfig searches for .bundleradar.yml in the directory tree.
func FindAndLoadConfig(startDir string) (*Config, string, error) {
	if startDir == "" {
		var err error
		startDir, err = os.Getwd()
		if err != nil {
			return nil, "", err
		}
	}

	curr := startDir
	for {
		candidate := filepath.Join(curr, ".bundleradar.yml")
		if _, err := os.Stat(candidate); err == nil {
			cfg, err := LoadConfig(candidate)
			return cfg, candidate, err
		}
		candidateYaml := filepath.Join(curr, ".bundleradar.yaml")
		if _, err := os.Stat(candidateYaml); err == nil {
			cfg, err := LoadConfig(candidateYaml)
			return cfg, candidateYaml, err
		}

		parent := filepath.Dir(curr)
		if parent == curr {
			return nil, "", nil
		}
		curr = parent
	}
}
