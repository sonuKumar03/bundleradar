package workspaces

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sonuKumar03/bundleradar/internal/core"
)

// NxResolver discovers projects in Nx workspaces without calling the Nx CLI.
type NxResolver struct{}

func (r *NxResolver) Name() string {
	return "nx"
}

func (r *NxResolver) Detect(root string) bool {
	_, err := os.Stat(filepath.Join(root, "nx.json"))
	return err == nil
}

func (r *NxResolver) Resolve(ctx context.Context, root string) ([]core.Target, error) {
	// Look inside apps/
	appsDir := filepath.Join(root, "apps")
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		return nil, nil
	}

	var targets []core.Target
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirName := entry.Name()
		name := dirName
		appPath := filepath.Join(appsDir, dirName)
		outputPath := ""
		projectData, err := os.ReadFile(filepath.Join(appPath, "project.json"))
		if err == nil {
			var project struct {
				Name        string `json:"name"`
				ProjectType string `json:"projectType"`
				Targets     map[string]struct {
					Options struct {
						OutputPath string `json:"outputPath"`
					} `json:"options"`
					DefaultConfiguration string `json:"defaultConfiguration"`
					Configurations       map[string]struct {
						OutputPath string `json:"outputPath"`
					} `json:"configurations"`
				} `json:"targets"`
			}
			if err := json.Unmarshal(projectData, &project); err != nil {
				return nil, fmt.Errorf("parse Nx project config %q: %w", filepath.Join(appPath, "project.json"), err)
			}
			if project.ProjectType == "library" {
				continue
			}
			if project.Name != "" {
				name = project.Name
			}
			if build, ok := project.Targets["build"]; ok {
				outputPath = build.Options.OutputPath
				if config, ok := build.Configurations[build.DefaultConfiguration]; ok && config.OutputPath != "" {
					outputPath = config.OutputPath
				}
			}
			outputPath = strings.ReplaceAll(outputPath, "{workspaceRoot}", "")
			outputPath = strings.ReplaceAll(outputPath, "{projectRoot}", filepath.Join("apps", dirName))
		} else if !os.IsNotExist(err) {
			return nil, err
		}

		// Look for the configured output first, then conventional layouts.
		statsCandidates := []string{}
		if outputPath != "" {
			outputDir := outputPath
			if !filepath.IsAbs(outputDir) {
				outputDir = filepath.Join(root, outputDir)
			}
			statsCandidates = append(statsCandidates,
				filepath.Join(outputDir, "stats.json"),
				filepath.Join(outputDir, "browser", "stats.json"),
				filepath.Join(outputDir, "metafile.json"),
				filepath.Join(outputDir, "manifest.json"),
				filepath.Join(outputDir, ".vite", "manifest.json"),
			)
		}
		statsCandidates = append(statsCandidates,
			filepath.Join(root, "dist", "apps", dirName, "stats.json"),
			filepath.Join(root, "dist", "apps", dirName, "browser", "stats.json"),
			filepath.Join(root, "dist", "apps", dirName, "metafile.json"),
			filepath.Join(root, "dist", "apps", dirName, "manifest.json"),
			filepath.Join(root, "dist", "apps", dirName, ".vite", "manifest.json"),
			filepath.Join(appPath, "dist", "stats.json"),
			filepath.Join(appPath, "stats.json"),
			filepath.Join(appPath, "dist", "metafile.json"),
			filepath.Join(appPath, "dist", "manifest.json"),
			filepath.Join(appPath, "dist", ".vite", "manifest.json"),
		)

		foundStats := ""
		foundDist := ""
		for _, sc := range statsCandidates {
			if _, err := os.Stat(sc); err == nil {
				foundStats = sc
				foundDist = filepath.Dir(sc)
				browserSub := filepath.Join(foundDist, "browser")
				if fi, err := os.Stat(browserSub); err == nil && fi.IsDir() {
					foundDist = browserSub
				}
				break
			}
		}

		if foundStats == "" {
			// Even if unbuilt, record project target
			if outputPath != "" {
				if !filepath.IsAbs(outputPath) {
					outputPath = filepath.Join(root, outputPath)
				}
				foundStats = filepath.Join(outputPath, "stats.json")
				foundDist = filepath.Join(outputPath, "browser")
			} else {
				foundStats = filepath.Join(root, "dist", "apps", dirName, "stats.json")
				foundDist = filepath.Join(root, "dist", "apps", dirName, "browser")
			}
		}

		targets = append(targets, core.Target{
			Name:      name,
			StatsPath: foundStats,
			DistPath:  foundDist,
		})
	}

	return targets, nil
}
