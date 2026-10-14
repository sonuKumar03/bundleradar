# BundleRadar Cheatsheet

## Scan & Inspect

```bash
# Basic scan — auto-detects bundler format
bundleradar scan dist/my-app/stats.json

# Scope to a specific entrypoint
bundleradar scan dist/my-app/stats.json --entry src/main.ts

# Trace why a package is in the bundle
bundleradar scan dist/my-app/stats.json --why lodash-es

# Show top 15 contributors
bundleradar scan dist/my-app/stats.json --top 15

# JSON output for scripts or agents
bundleradar scan dist/my-app/stats.json --format json -o scan.json
```

## Compare & Diff

```bash
# Diff against a baseline file
bundleradar diff dist/my-app/stats.json --against baseline.json

# Diff against a git ref (builds base in a worktree)
bundleradar diff dist/my-app/stats.json --against main

# PR markdown report
bundleradar diff dist/my-app/stats.json --against main -f github-pr -o report.md

# Ignore small changes below threshold
bundleradar diff dist/my-app/stats.json --against main --drift-threshold 1KB
```

## CI Budget Gate

```bash
# Enforce JS size limits
bundleradar gate dist/my-app/stats.json --max-initial 250kb --max-total 1.5mb

# Enforce CSS and asset budgets
bundleradar gate dist/my-app/stats.json --max-css 100kb --max-assets 2mb

# Block regressions vs baseline
bundleradar gate dist/my-app/stats.json --against main --max-initial-delta 0kb

# Forbid packages and detect duplicates
bundleradar gate dist/my-app/stats.json --forbid moment,lodash --detect-duplicate-pkgs
```

## Monorepo

```bash
# List all app targets
bundleradar workspace list --root .

# Scan all apps
bundleradar workspace scan --root .

# Scan specific apps
bundleradar workspace scan --app "portal=dist/portal/stats.json" --app "admin=dist/admin/stats.json"
```

## Interactive Studio UI

```bash
# Open web UI with treemap, trends, and live file watching
bundleradar ui dist/my-app/stats.json
```

## Build History

```bash
# Record a checkpoint
bundleradar history save dist/my-app/stats.json --label "v2.4.1"

# List all checkpoints
bundleradar history list

# Show details for a checkpoint
bundleradar history show <id>
```

## MCP Server (AI Agents)

```bash
# Start MCP server for Claude Code, Cursor, etc.
bundleradar mcp

# Add to Claude Code
claude mcp add bundleradar -- bundleradar mcp
```

## Config File (`.bundleradar.yml`)

```yaml
budgets:
  initial_js_max: 250kb
  total_max: 1.5mb
  max_css: 100kb
  max_assets: 2mb

rules:
  disallow_packages:
    - moment
    - lodash
```

CLI flags override config values.

## GitHub Action

```yaml
- uses: sonuKumar03/bundleradar@v2.4.1
  with:
    stats: dist/my-app/stats.json
    entry: src/main.ts
    max-initial: '250kb'
    max-total: '1.2mb'
    max-initial-delta: '0B'
    post-comment: true           # sticky PR comment
    artifact-baseline: true      # skip rebuilding base branch
    upload-artifact-baseline: true  # save baseline on push to main
    project: portal              # namespace for monorepos
```

## Exit Codes

| Code | Meaning |
|:---:|:---|
| `0` | Pass (or no rules configured — report-only) |
| `1` | Budget or rule violated |
| `2` | Bad config or CLI flags |
| `3` | Can't read input (missing file, bad JSON) |

## Supported Bundlers

| Bundler | Stats file | Build command |
|:---|:---|:---|
| Angular | `stats.json` | `ng build --stats-json` |
| esbuild | `meta.json` | `esbuild --metafile=meta.json` |
| Vite | `manifest.json` | `npx vite build -- --stats` |
| Webpack | `stats.json` | `npx webpack --json=stats.json` |

All formats are auto-detected — no `--bundler` flag needed in most cases.
