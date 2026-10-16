<div align="center">

# ⚡ bundleradar

**Lightning-fast bundle inspector, dependency tracer, optimization advisor, and CI budget gate for Angular, esbuild, Vite, and Webpack.**

<br>

[![Release](https://img.shields.io/github/v/release/sonuKumar03/bundleradar?color=indigo&label=release&logo=github)](https://github.com/sonuKumar03/bundleradar/releases)
[![GitHub Action](https://img.shields.io/badge/GitHub%20Action-Ready-2088FF?logo=githubactions&logoColor=white)](#-ci--github-actions-integration)
[![CI Status](https://img.shields.io/github/actions/workflow/status/sonuKumar03/bundleradar/ci.yml?branch=master&label=CI&logo=githubactions)](https://github.com/sonuKumar03/bundleradar/actions)
[![Go Report](https://img.shields.io/badge/Go-1.27.1+-00ADD8?style=flat&logo=go)](https://go.dev)
[![Platforms](https://img.shields.io/badge/platforms-macOS%20%7C%20Linux%20%7C%20Windows-blue)](https://github.com/sonuKumar03/bundleradar/releases)
[![Agent Skill](https://img.shields.io/badge/AI%20Skill-Ready-8A2BE2?style=flat&logo=anthropic)](.agents/skills/bundleradar/SKILL.md)
[![License](https://img.shields.io/github/license/sonuKumar03/bundleradar?color=emerald)](LICENSE)

<br>

[**Website & Live Docs**](https://sonukumar03.github.io/bundleradar/) • [**Cheatsheet**](docs/cheatsheet.md) • [**Why bundleradar?**](#-why-bundleradar) • [**Installation**](#-installation) • [**Quick Start**](#-quick-start) • [**Command Reference**](#-command-reference) • [**CI & GitHub Actions**](#-ci--github-actions-integration) • [**AI Agent Skill**](#-ai-agent-skill-integration)

</div>

---

## ⚡ Overview

JavaScript bundlers produce stats files that are massive, complex, and unreadable for quick human inspection or CI pull request reviews — whether it's esbuild's `meta.json`, Webpack's `stats.json`, Vite's `rollup-stats.json`, or Angular's build output.

`bundleradar` is a self-contained Go binary with zero runtime dependencies that turns any bundler's stats output into **actionable dependency hierarchies, file-by-file root cause traces, automated optimization suggestions, and hard CI budget gates**. Supports **Angular, esbuild, Vite, and Webpack** out of the box. Fast native analysis; reported timings exclude builds and external subprocesses.

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

**Install** (macOS, Linux, Windows WSL):

```bash
curl -fsSL https://raw.githubusercontent.com/sonuKumar03/bundleradar/master/install.sh | sh
```

**Update:**

```bash
bundleradar update
```

`bundleradar update --check` only reports whether a newer release exists, and `--version v2.5.1` installs a specific release. Downloads are verified against the release's `checksums.txt` before the binary is replaced. In an interactive terminal, bundleradar mentions a newer release at most once every 7 days; set `BUNDLERADAR_NO_UPDATE_CHECK=1` to turn that off. It never runs in CI, when output is piped, or for `mcp`.

**Uninstall** (removes the binary and any installed agent skills):

```bash
curl -fsSL https://raw.githubusercontent.com/sonuKumar03/bundleradar/master/uninstall.sh | sh
```

Precompiled binaries for all platforms are on [**GitHub Releases**](https://github.com/sonuKumar03/bundleradar/releases/latest).

---

## 🚀 Quick Start

### 1. Build your application with stats

Generate a stats file for your bundler:

**Angular**

```bash
ng build --configuration production --stats-json
```

**esbuild**

```bash
esbuild src/index.ts --bundle --metafile=meta.json --outdir=dist
```

**Vite**

```bash
npx vite build -- --stats
```

**Webpack**

```bash
npx webpack --json=stats.json
```
*This produces a stats/meta JSON file that bundleradar auto-detects.*

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

| Command | Description |
|:---|:---|
| `bundleradar scan <stats>` | Inspect bundle sizes, breakdown, top npm packages, and trace imports with `--why` |
| `bundleradar diff <stats> --against <ref>` | Compare against a baseline file or git ref with regression attribution |
| `bundleradar gate <stats> --max-initial 250kb` | CI budget gate — exits `1` on violation. Supports JS/CSS/asset/delta limits and `--forbid` |
| `bundleradar workspace scan --root .` | Discover and analyze all apps in a monorepo (Nx, pnpm, npm, yarn) |
| `bundleradar mcp` | Launch MCP server for AI coding agents (Claude Code, Cursor, Claude Desktop) |
| `bundleradar history save/list/show` | Record and inspect persistent build history checkpoints |
| `bundleradar ui` | Open interactive Studio UI with treemap, trends, and live file watching |
| `bundleradar update` | Update to the latest release (`--check` to only look, `--version` to pin) |

All commands support `--format json` for machine-readable output. `--entry` / `-e` scopes initial/lazy reachability to a source path or exact chunk name (globs are not supported); `TotalJS` always reflects the whole browser build. See the [**full docs**](https://sonukumar03.github.io/bundleradar/) for detailed flags and examples.

---

## 🛡️ CI & GitHub Actions Integration

Add automated bundle size gating and PR delta comments with the official action:

```yaml
- name: Bundle Size Guard
  uses: sonuKumar03/bundleradar@v2.7.0
  with:
    stats: dist/my-app/stats.json
    entry: src/main.ts
    max-initial: '250kb'
    max-total: '1.2mb'
    max-initial-delta: '0B'
    post-comment: true
```

Supports artifact-based baselines, monorepo project namespacing, and build history. See [action.yml](action.yml) for all inputs.

---

## ⚙️ Configuration (`.bundleradar.yml`)

Persist size budgets and disallowed-package rules at the project root. CLI flags override config values.

```yaml
# .bundleradar.yml
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

Exit codes: `0` pass, `1` policy violation, `2` config error, `3` execution failure.

---

## 🤖 AI Agent Integration

Add bundleradar as an MCP server for AI coding agents:

```bash
claude mcp add bundleradar -- bundleradar mcp
```

Or install the binary with the companion [Agent Skill](.agents/skills/bundleradar/SKILL.md):
```bash
curl -fsSL https://raw.githubusercontent.com/sonuKumar03/bundleradar/master/install.sh | sh -s -- --with-skill
```

For MCP config, agent contracts, and optimization workflows, see [**docs/agents.md**](docs/agents.md).

---

## 🏗️ Architecture

```text
stats.json / meta.json ──────────────┐
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
# Run test suite (154 tests across 13 packages)
go test -v ./...

# Run linter
go vet ./...

# Build binary
go build -o bundleradar .
```

---

## 📄 License

Released under the **MIT License**. Built with ❤️ for the JavaScript developer performance community.

