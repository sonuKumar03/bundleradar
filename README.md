<div align="center">

# ⚡ bundleradar

**Lightning-fast bundle inspector, dependency tracer, optimization advisor, and CI budget gate for Angular esbuild.**

<br>

[![Release](https://img.shields.io/github/v/release/sonuKumar03/bundleradar?color=indigo&label=release&logo=github)](https://github.com/sonuKumar03/bundleradar/releases)
[![GitHub Action](https://img.shields.io/badge/GitHub%20Action-Ready-2088FF?logo=githubactions&logoColor=white)](#-ci--github-actions-integration)
[![CI Status](https://img.shields.io/github/actions/workflow/status/sonuKumar03/bundleradar/ci.yml?branch=master&label=CI&logo=githubactions)](https://github.com/sonuKumar03/bundleradar/actions)
[![Go Report](https://img.shields.io/badge/Go-1.27.1+-00ADD8?style=flat&logo=go)](https://go.dev)
[![Platforms](https://img.shields.io/badge/platforms-macOS%20%7C%20Linux%20%7C%20Windows-blue)](https://github.com/sonuKumar03/bundleradar/releases)
[![Agent Skill](https://img.shields.io/badge/AI%20Skill-Ready-8A2BE2?style=flat&logo=anthropic)](.agents/skills/bundleradar/SKILL.md)
[![License](https://img.shields.io/github/license/sonuKumar03/bundleradar?color=emerald)](LICENSE)

<br>

[**Website & Live Docs**](https://sonukumar03.github.io/bundleradar/) • [**Why bundleradar?**](#-why-bundleradar) • [**Installation**](#-installation) • [**Quick Start**](#-quick-start) • [**Command Reference**](#-command-reference) • [**CI & GitHub Actions**](#-ci--github-actions-integration) • [**AI Agent Skill**](#-ai-agent-skill-integration)

</div>

---

## ⚡ Overview

Modern Angular applications build with **esbuild** for incredible compilation speed. However, esbuild's raw `stats.json` files are massive, complex, and unreadable for quick human inspection or CI pull request reviews.

`bundleradar` is a self-contained Go binary with zero runtime dependencies that turns Angular `stats.json` files into **actionable dependency hierarchies, file-by-file root cause traces, automated optimization suggestions, and hard CI budget gates**. Fast native analysis; reported timings exclude Angular builds and external Nx subprocesses.

```text
$ bundleradar diff dist/apps/portal/stats.json --against origin/master

⚡ BUNDLERADAR COMPARISON & DIFF
-------------------------------------------------------------
Initial JS:   2.79 MB → 3.08 MB (+295.73 KB)
Lazy JS:      87.13 KB → 87.13 KB (0 B)
Total JS:     2.88 MB → 3.17 MB (+295.73 KB)

CHANGED PACKAGES
-------------------------------------------------------------
➕ three                  +295.20 KB (main-PJ5NP3ZD.js)
   • Import path: apps/portal/src/main.ts → apps/portal/src/app/app.component.ts → node_modules/three/build/three.module.js
🔄 @angular/router            +244 B (main-PJ5NP3ZD.js)
   • Import path: apps/portal/src/main.ts → apps/portal/src/app/app.config.ts → node_modules/@angular/router/fesm2022/router.mjs
🔄 lodash                      +11 B (main-PJ5NP3ZD.js)
   • Import path: apps/portal/src/main.ts → apps/portal/src/app/app.component.ts → node_modules/lodash/cloneDeep.js
🔄 @angular/platform-browser       +2 B (chunk-IF6QX6MW.js, main-PJ5NP3ZD.js)
   • Import path: apps/portal/src/main.ts → node_modules/@angular/platform-browser/fesm2022/platform-browser.mjs

Micro-drift: 70.31 KB across sub-threshold updates
```

---

## 🥊 Why bundleradar?

| Feature | `bundleradar` | `webpack-bundle-analyzer` | `source-map-explorer` | Standard `angular.json` Budgets |
| :--- | :---: | :---: | :---: | :---: |
| **Execution** | **Fast native analysis** | Node.js process | Node.js process | Integrated into build |
| **Runtime Dependencies** | **Zero** (Self-contained binary) | ~40+ npm packages | ~30+ npm packages | Node.js |
| **Import Chain Tracer (`--why`)** | **Yes (Directed BFS Trail)** | ❌ No | ❌ No | ❌ No |
| **Regression Attribution** | **Yes (Package & Chunk level)** | ❌ No (Visual only) | ❌ No | ❌ No |
| **Gzip Wire Modeling** | **Yes (Built-in estimation)** | Yes | Yes | ❌ Raw bytes only |
| **PR Delta Diffs (`diff`)** | **Yes (Signed +/- KB & PR Markdown)** | ❌ No | ❌ No | ❌ No |
| **Headless CI Gating (`gate`)** | **Yes (Exit 0/1 automation codes)** | ❌ GUI Required | ❌ GUI / HTML | Yes (Limited) |
| **AI Coding Agent Skill & MCP** | **Yes (`bundleradar mcp`)** | ❌ No | ❌ No | ❌ No |

---

## 📦 Installation

### Option 1: 1-Line Standalone Shell Installer

Downloads the latest precompiled native binary to `$GOBIN` when set, otherwise `/usr/local/bin` (or `~/.local/bin`). No Go installation is needed for release binaries. Running the installer from a source checkout builds that checkout using Go:

```bash
curl -fsSL https://raw.githubusercontent.com/sonuKumar03/bundleradar/master/install.sh | sh
```

*To install the binary alongside the AI Agent skill:*
```bash
curl -fsSL https://raw.githubusercontent.com/sonuKumar03/bundleradar/master/install.sh | sh -s -- --with-skill
```

### Option 2: Precompiled Multi-Arch Binaries

Download standalone binaries directly from the [**GitHub Releases**](https://github.com/sonuKumar03/bundleradar/releases/latest):
- 🍏 **macOS Apple Silicon (M1/M2/M3/M4)**: `bundleradar_*_darwin_arm64.tar.gz`
- 🍏 **macOS Intel**: `bundleradar_*_darwin_amd64.tar.gz`
- 🐧 **Linux x86_64**: `bundleradar_*_linux_amd64.tar.gz`
- 🐧 **Linux ARM64**: `bundleradar_*_linux_arm64.tar.gz`
- 🪟 **Windows x64**: `bundleradar_*_windows_amd64.zip`

### Option 3: Go Install

```bash
go install github.com/sonuKumar03/bundleradar@latest
```

### Clean Uninstall

To cleanly remove `bundleradar` (and legacy `bundlecheck`) binaries and installed companion agent skills:

```bash
curl -fsSL https://raw.githubusercontent.com/sonuKumar03/bundleradar/master/uninstall.sh | sh
```

---

## 🚀 Quick Start

### 1. Build your Angular application with stats

Add `--stats-json` to your build command (or configure `"statsJson": true` in `angular.json`):

```bash
ng build --configuration production --stats-json
```
*This produces `dist/<project-name>/stats.json`.*

### 2. Inspect with bundleradar

```bash
# View initial JS vs lazy breakdown & top npm contributors
bundleradar scan dist/my-app/stats.json

# Match a source entry or exact emitted chunk name (globs are not supported)
bundleradar scan dist/my-app/stats.json --entry src/main.ts
bundleradar scan dist/my-app/stats.json -e "main.js"

# Trace why a package was pulled into initial JS
bundleradar scan dist/my-app/stats.json --why lodash-es

# Compare against a baseline file or git ref with regression attribution
bundleradar diff dist/my-app/stats.json --against main

# Enforce CI size budget (fails with exit code 1 on violation)
bundleradar gate dist/my-app/stats.json --max-initial 250kb --max-total 1.2mb
```

---

## 📖 Command Reference

### 1. `bundleradar scan`
Inspect bundle sizes, breakdown, and package dependencies across entrypoints. Supports esbuild, Angular, Vite, and Webpack stats.

```bash
# Basic scan
bundleradar scan dist/my-app/stats.json

# Scope to a source path or exact emitted chunk name (globs are not supported)
bundleradar scan dist/my-app/stats.json --entry src/main.ts
bundleradar scan dist/my-app/stats.json -e "main.js"

# Show top 15 packages and trace package dependency root
bundleradar scan dist/my-app/stats.json --top 15 --why lodash-es

# Export machine-readable JSON (ideal for scripts & agent loops)
bundleradar scan dist/my-app/stats.json --format json -o scan.json
```

> **Entrypoint selection:** `--entry` / `-e` accepts a source path or exact emitted chunk name; glob patterns are not supported. It scopes analysis where the stats expose separate entrypoints. Angular currently exposes a single `main` entrypoint, so this does not isolate individual lazy routes. `TotalJS` always reflects the whole browser build.

---

### 2. `bundleradar diff`
Compare current build against a baseline file or git ref with regression attribution. Automatically creates an isolated temporary git worktree and runs `--build-cmd` when given a git ref.

```bash
# Compare against baseline file
bundleradar diff dist/my-app/stats.json --against baseline.json

# Compare against git branch with worktree build and drift threshold
bundleradar diff dist/my-app/stats.json --against main --drift-threshold 1KB

# Generate PR markdown report for CI
bundleradar diff dist/my-app/stats.json --against main -f github-pr -o report.md
```

**Sample Generated PR Markdown Comment:**

```markdown
<!-- bundleradar-report -->
## ⚡ BundleRadar Comparison & Diff

| Category | Before | After | Delta | Status |
| :--- | :---: | :---: | :---: | :---: |
| **Initial JS** | `2.79 MB` | `3.08 MB` | **+295.73 KB** | ⚠️ Increased |
| **Lazy JS** | `87.13 KB` | `87.13 KB` | **0 B** | ⚪ Neutral |
| **Total JS** | `2.88 MB` | `3.17 MB` | **+295.73 KB** | ⚠️ Increased |

### 🔎 Regression Explanation

- 📦 **`three`** (`+295.20 KB`) → emitted in `main-PJ5NP3ZD.js`
  - **Import path:** `apps/portal/src/main.ts → apps/portal/src/app/app.component.ts → node_modules/three/build/three.module.js`

### Changed Packages

| Package | Status | Delta | Base Size | Current Size |
| :--- | :---: | :---: | :---: | :---: |
| `three` | ➕ Added | `+295.20 KB` | `0 B` | `295.20 KB` |
| `@angular/router` | 🔄 Changed | `+244 B` | `81.32 KB` | `81.56 KB` |
| `lodash` | 🔄 Changed | `+11 B` | `18.27 KB` | `18.28 KB` |
```

---

### 3. `bundleradar gate`
Validate bundle sizes, regressions, and architecture rules against policy budgets in CI.

```bash
# Enforce initial and total JS size limits
bundleradar gate dist/my-app/stats.json --max-initial 250kb --max-total 1.5mb

# Enforce regression limits against a baseline file or git ref
bundleradar gate dist/my-app/stats.json --against main --max-initial-delta 0kb

# Disallow unwanted packages and detect duplicate package copies
bundleradar gate dist/my-app/stats.json --forbid moment,lodash --detect-duplicate-pkgs
```
*Exits with status `0` on success, or status `1` when any budget or rule is violated.*

---

### 4. `bundleradar workspace`
Discover and analyze applications across monorepos and multi-app workspaces (Nx, pnpm, npm, yarn).

```bash
# List all discovered application targets
bundleradar workspace list --root .

# Scan and summarize all application targets
bundleradar workspace scan --root .

# Scan specific application targets
bundleradar workspace scan --app "portal=apps/portal/dist/stats.json" --app "admin=apps/admin/dist/stats.json" -f json
```

---

### 5. `bundleradar mcp` (Model Context Protocol Server for AI Agents)

Launch a native [Model Context Protocol (MCP)](https://modelcontextprotocol.io) server over standard I/O for AI coding assistants (**Claude Code**, **Antigravity**, **Cursor**, **Claude Desktop**).

```bash
bundleradar mcp
```

**Exposed MCP Tools:**
- `bundle_scan`: Analyze bundle sizes, entrypoints, and top contributing npm packages.
- `bundle_diff`: Compare against a local stats/metafile/manifest file or BundleRadar scan JSON file. MCP tools do not resolve git refs.
- `bundle_gate`: Validate bundle sizes, regressions, and architecture rules against policy budgets; `against` accepts a local baseline file, not a git ref.
- `workspace_summary`: List discovered application targets and their stats/dist paths. It does not scan bundle contents.

**Exposed MCP Resource:**
- `bundleradar://rules`: Bundle optimization rules and guidelines.

---

## 🛡️ CI & GitHub Actions Integration

### Official GitHub Action (`uses: sonuKumar03/bundleradar@v2.1.0`)

Add automated bundle size budget validation and PR delta comments to `.github/workflows/bundle-size.yml`:

```yaml
name: Bundle Size Guard
on: [pull_request]

permissions:
  contents: read
  pull-requests: write

jobs:
  bundleradar:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: 20
          cache: 'npm'

      - run: npm ci
      - run: npx ng build --configuration production --stats-json

      - name: Run bundleradar & Post PR Report
        uses: sonuKumar03/bundleradar@v2.1.0
        with:
          stats: dist/my-app/stats.json
          entry: src/main.ts
          max-initial: '250kb'
          max-total: '1.2mb'
          max-initial-delta: '0B'
          post-comment: true
```

#### Action Inputs & Comparison Behavior

| Input | Default | Description |
|:---|:---:|:---|
| `stats` | *(auto)* | Path to `stats.json` (auto-detected if omitted). |
| `dist` | *(auto)* | Path to emitted `browser` dist with `index.html` (auto-detected if omitted). |
| `project` | `""` | Project name for multi-project or Nx workspaces. |
| `entry` | `""` | Match a source path or exact emitted chunk name (e.g. `src/main.ts` or `main.js`); glob patterns are not supported. |
| `artifact-baseline` | `false` | Attempt to restore baseline summary JSON from a GitHub Actions workflow artifact on base-ref. |
| `artifact-name` | `""` | Name of the baseline workflow artifact (defaults to `bundleradar-baseline` or `bundleradar-baseline-<project>`). |
| `upload-artifact-baseline` | `false` | Save current bundle summary and upload as an immutable baseline workflow artifact. |
| `github-token` | `github.token` | Token used for downloading baseline artifacts and posting PR comments. |
| `base-ref` | `github.base_ref` | Git ref for baseline comparison in PRs. Automatically fetched in shallow checkouts (`fetch-depth: 1` or `0`). |
| `build-cmd` | `"npm run build"` | Command used to build `base-ref` inside an isolated temporary git worktree. |
| `max-initial-delta` | `""` | Maximum allowed increase in initial JS vs baseline (e.g. `0B`, `10KB`). |
| `max-total-delta` | `""` | Maximum allowed increase in total JS vs baseline. |
| `post-comment` | `false` | Automatically creates or updates a single sticky PR comment with visual diffs. Requires `pull-requests: write`. |

#### Persistent Memory with Workflow Artifacts (Fast & Secure)
Instead of rebuilding the base branch in an isolated Git worktree for every PR, you can enable persistent baseline memory:
1. **On `push` to `main`:** Set `upload-artifact-baseline: true` to save and upload the baseline snapshot as a GitHub workflow artifact.
2. **On `pull_request`:** Set `artifact-baseline: true` to automatically download the immutable baseline artifact using GitHub CLI. If the artifact is not found, it gracefully falls back to the Git worktree build.
3. **Multi-App Monorepos:** In multi-app workspaces, specifying `project: my-app` automatically namespaces the artifact to `bundleradar-baseline-my-app`, enabling safe parallel matrix builds across applications.

**Fallback & Error Handling:**
- A successful baseline comparison produces the diff report and marks `comparison-status` as `completed`.
- If an artifact is missing, the Action tries the configured or pull request base ref; without either, it scans the current build and reports comparison status as `unavailable`.
- If a requested baseline comparison fails, the Action fails instead of silently falling back to a current-only scan, and it does not report the comparison as completed.

### ☁️ On-Demand Remote Audits
Audit any public open-source Angular repository directly via GitHub Actions without local installation:
1. Navigate to **Actions** → **Remote Bundle Audit**.
2. Click **Run workflow** and input the public repository URL (e.g. `https://github.com/user/angular-app`).
3. View the full bundle breakdown, top packages, and optimization recommendations directly in the **Job Summary**.

---

## ⚙️ Configuration (`.bundleradar.yml`)

Persist size budgets and disallowed-package rules at the root of your project. `gate` loads the nearest configuration in the current directory or a parent directory; CLI flags override corresponding configuration values. Invalid YAML and unsupported settings fail with exit code 2:

```yaml
# .bundleradar.yml
budgets:
  initial_js_max: 250kb
  total_max: 1.5mb

rules:
  disallow_packages:
    - moment
    - lodash
```

### Budget Precedence Table

bundleradar enforces limits strictly according to the following deterministic precedence:

| Priority | Source | Description |
|:---:|:---|:---|
| **1 (Highest)** | **CLI Flags** | Explicit command-line arguments (e.g., `--max-initial 200KB`, `--max-total 1MB`) override all configuration values. |
| **2** | **Auto-Loaded Config** | Automatically discovered `.bundleradar.yml` / `.bundleradar.yaml` in current or parent directory. |
| **3 (Lowest)** | **No Limit** | Default for unconfigured projects: report-only mode with zero invented failures. |

### Outcome Semantics

| Outcome | Exit Status | Description |
|:---|:---:|:---|
| **Pass** | `0` | All configured budgets and package rules passed, or project was evaluated in unconfigured report-only mode. Full metrics and summary are output. |
| **Policy Violation** | `1` | One or more thresholds or disallowed package rules were breached. Detailed violations are printed alongside actual vs limit values, while preserving full bundle breakdown. |
| **Usage / Config Error** | `2` | Invalid CLI flags, missing required arguments, malformed YAML, unparseable byte values, or unknown configuration fields are rejected before analysis begins. |
| **Execution Failure** | `3` | Missing build artifacts, unparseable stats JSON, I/O errors, or missing baseline file prevents analysis from completing. |

---

## 🤖 AI Agent Integration

Give your coding agent a measured bundle optimization loop:

**baseline → diagnose → trace → edit → rebuild/test → measure → gate**

- **Agent Skill**: the reasoning and optimization workflow.
- **MCP**: the preferred structured tool interface when available.
- **CLI JSON**: the universal fallback, using `--format json`.

The installed `bundleradar` binary and Angular esbuild `stats.json` are prerequisites. The Skill chooses MCP or CLI transport without changing the workflow.

### MCP server

Add the preferred structured interface to your agent's MCP configuration:

**Claude Code:**
```bash
claude mcp add bundleradar -- bundleradar mcp
```

**Claude Desktop / Cursor (`claude_desktop_config.json` / `.cursor/mcp.json`):**
```json
{
  "mcpServers": {
    "bundleradar": {
      "command": "bundleradar",
      "args": ["mcp"]
    }
  }
}
```

### Agent Skill ([`SKILL.md`](.agents/skills/bundleradar/SKILL.md))

Install the binary and complete skill tree for generic agents, Claude Code, and Codex:
```bash
curl -fsSL https://raw.githubusercontent.com/sonuKumar03/bundleradar/master/install.sh | sh -s -- --with-skill
```

Use `--skill-dir <path>` to install only to an explicit custom skill location.

Try this prompt:

> Reduce the initial JavaScript bundle by at least 50 KB without changing
> application behavior. Establish a baseline first, identify the highest-confidence
> optimization, trace its import path, make the change, rebuild, run tests, and
> report the measured delta.

For complete agent documentation and JSON contracts, see [**docs/agents.md**](docs/agents.md).

---

## 🏗️ Architecture

```text
Angular esbuild stats.json ──────────┐
                                     ▼
browser/ dist + index.html ──► internal/adapters/parsers (Angular, Esbuild, Vite, Webpack)
                                     │
                                     ▼
                             internal/core (Bundle AST, BFS Traversal, Gzip Estimation)
                                     │
                  ┌──────────────────┼──────────────────┐
                  ▼                  ▼                  ▼
         internal/core/diff   internal/core/policy   internal/adapters/workspaces
        (Delta Attribution)    (Budget Gate)           (Nx Monorepo Discovery)
                  │                  │                  │
                  └──────────────────┼──────────────────┘
                                     ▼
                     internal/adapters/reporters
                  ┌──────────┬───────────┬──────────┐
                  ▼          ▼           ▼          ▼
            Terminal       JSON      Markdown   GitHub PR
          (CLI Output)  (AI Agents)  (Reports)  (PR Comments)
```

---

## 🧪 Development & Testing

```bash
# Run test suite (71 tests across 11 packages)
go test -v ./...

# Run linter
go vet ./...

# Build binary
go build -o bundleradar .
```

---

## 📄 License

Released under the **MIT License**. Built with ❤️ for the Angular & developer performance community.

---

## 🙌 Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, test/lint
commands, and release process. Release history is in [CHANGELOG.md](CHANGELOG.md).
