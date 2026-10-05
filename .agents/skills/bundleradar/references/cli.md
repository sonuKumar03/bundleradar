# CLI fallback

Use `--format json` for agent decisions. BundleRadar parses bundle outputs (stats.json, metafile.json); build the application first if artifacts are missing or stale.

## Install and discover

```sh
command -v bundleradar
curl -fsSL https://raw.githubusercontent.com/sonuKumar03/bundleradar/master/install.sh | sh
```

The release installer downloads a prebuilt binary; Go is not required. BundleRadar supports esbuild, Angular, Vite, and Webpack stats.

## Commands

```sh
# Scan bundle sizes, entrypoints, and top package contributions
bundleradar scan dist/my-app/stats.json -f json
bundleradar scan dist/my-app/stats.json --entry src/main.ts -f json
bundleradar scan dist/my-app/stats.json --why lodash -f json

# Compare regressions against baseline file or git ref with worktree build
bundleradar diff dist/my-app/stats.json --against baseline.json -f json
bundleradar diff dist/my-app/stats.json --against main --drift-threshold 1KB -f json

# Gate bundle budgets and architecture rules in CI
bundleradar gate dist/my-app/stats.json --max-initial 250KB --max-total 1MB -f json
bundleradar gate dist/my-app/stats.json --max-css 100KB --max-assets 2MB -f json
bundleradar gate dist/my-app/stats.json --against main --max-initial-delta 10KB -f json
bundleradar gate dist/my-app/stats.json --forbid moment,lodash -f json

# Discover and scan multi-app workspaces and monorepos
bundleradar workspace list --root .
bundleradar workspace scan --app "app1=apps/app1/stats.json" --app "app2=apps/app2/stats.json" -f json
```

## Failures

Capture stdout, stderr, and exit status separately.
Exit codes:
- `0`: Success (analysis passed, policies met).
- `1`: Policy violation (budget breached or forbidden package detected).
- `2`: Usage / configuration error (invalid flags or arguments).
- `3`: Execution failure (missing file, I/O error).
