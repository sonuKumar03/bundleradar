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
	Budgets ConfigBudgets `yaml:"budgets,omitempty"`
	Rules   ConfigRules   `yaml:"rules,omitempty"`
}

// ConfigBudgets holds threshold limits for bundle metrics and deltas.
type ConfigBudgets struct {
	InitialJSMax    string `yaml:"initial_js_max,omitempty"`
	TotalMax        string `yaml:"total_max,omitempty"`
	MaxInitialDelta string `yaml:"max_initial_delta,omitempty"`
	MaxCSS          string `yaml:"max_css,omitempty"`
	MaxAssets       string `yaml:"max_assets,omitempty"`
}

// ConfigRules holds architectural lint rules for bundled packages.
type ConfigRules struct {
	DisallowPackages []string `yaml:"disallow_packages,omitempty"`
}

// ToPolicy converts config budgets and rules to an evaluation Policy.
func (c *Config) ToPolicy() (Policy, error) {
	var pol Policy
	if c == nil {
		return pol, nil
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
