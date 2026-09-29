# Using bundleradar as an AI Coding Agent

Give your coding agent a measured bundle optimization loop for applications:

**scan → diff → gate**

- **Agent Skill**: the reasoning and optimization workflow.
- **MCP**: the preferred structured tool interface when available.
- **CLI JSON**: the universal fallback, using `--format json`.

The installed `bundleradar` binary and compatible `stats.json` are prerequisites. Install the binary and complete skill tree for generic agents, Claude Code, and Codex with:

```sh
curl -fsSL https://raw.githubusercontent.com/sonuKumar03/bundleradar/master/install.sh | sh -s -- --with-skill
```

Use `--skill-dir <path>` to install only to a custom skill location. Do not assume the BundleRadar source checkout is available.

Example prompt:

> Reduce the initial JavaScript bundle by at least 50 KB without changing
> application behavior. Establish a baseline first, identify the highest-confidence
> optimization, trace its import path, make the change, rebuild, run tests, and
> report the measured delta.

---

## 🔌 Model Context Protocol (MCP) Server

`bundleradar` includes a native, high-performance MCP server built into the binary:

```sh
bundleradar mcp
```

### Agent Client Configuration

#### 1. Claude Code
Add to your project's `.claude.json` or run:
```sh
claude mcp add bundleradar -- bundleradar mcp
```

#### 2. Antigravity / Gemini CLI
Add to `~/.gemini/antigravity-cli/mcp/bundleradar.json` (or project MCP settings):
```json
{
  "command": "bundleradar",
  "args": ["mcp"]
}
```

#### 3. Claude Desktop (`claude_desktop_config.json`)
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

#### 4. Cursor (`.cursor/mcp.json`)
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

---

### Exposed MCP Tools

The MCP server provides 4 typed tools formatted for LLM consumption:

| Tool | Parameters | Description |
| :--- | :--- | :--- |
| `bundle_scan` | `path` (required), `dist`, `bundler`, `top` | Inspect bundle sizes, breakdown, and package dependencies. |
| `bundle_diff` | `path` (required), `against` (required), `drift_threshold` | Compare against a local stats/metafile/manifest file or BundleRadar scan JSON file. Git refs are not supported by MCP. |
| `bundle_gate` | `path` (required), `against`, `max_initial`, `max_total`, `max_initial_delta`, `forbid` (array) | Validate bundle sizes against policy budgets in CI; `against` accepts a local baseline file, not a git ref. |
| `workspace_summary`| `root` | Lists discovered application target names and stats/dist paths; it does not scan bundle contents. |

### Exposed MCP Resources

- **`bundleradar://rules`** (`application/json`): Standard optimization heuristics, replacement guidelines for heavy libraries (`moment`, `lodash`, `exceljs`, `pdfjs-dist`), and default budget guidelines.

---

## ⚡ Auto-Discovery & Zero-Config CLI Usage

When executed in an application directory after a build that produces `stats.json`, `bundleradar` can automatically locate it and matching `dist` directories.

```sh
# Zero-config scan (JSON)
bundleradar scan --format json

# Multi-app workspace: scan
bundleradar workspace scan
```

---

## 🔍 Dependency Path Tracing (`why`)

Trace the import path explaining why a package or module is in the bundle:

```sh
# Trace dependency path to lodash
bundleradar scan stats.json --why lodash --format json
```

Inspect the output to see the sequence of source files leading from root entrypoints to the target package.

---

## 🏢 Multi-App Monorepos (`workspace`)

In Nx or multi-app monorepos:

```sh
# List apps
bundleradar workspace list

# Scan all applications
bundleradar workspace scan --format json

# Filter specific applications
bundleradar workspace scan --app portal=stats.json --format json
```

---

## 🔄 Iteration & Measurement Playbook for Agents

### 1. Establish Baseline Before Modifying Code
Baselines are handled seamlessly by `--against`. You can pass a path to a previous `stats.json` or a git ref (like `HEAD`).

### 2. Trace & Apply Optimization
1. Run `bundleradar scan stats.json --format json` to find high-impact targets.
2. Run `bundleradar scan stats.json --why <package> --format json` to identify the importing component.
3. Replace heavy modules with lazy loading.

### 3. Measure Changes & Verify
```sh
npm run build
bundleradar diff stats.json --against baseline.json --format json
```
Inspect the output to see size deltas.

### 4. Gate Regressions
```sh
# Fails with exit code 1 if initial JS grew
bundleradar gate stats.json --against baseline.json --max-initial-delta 0B
```

---

## 📋 JSON Schema Contracts

### Scan JSON (`command: "scan"`)

| Field | Meaning |
| :--- | :--- |
| `schemaVersion` | JSON contract version; currently `"1"`. |
| `toolVersion` | Tool release version; currently `"2.1.0"`. |
| `command` | The string `"scan"`. |
| `summary.initialJs`, `summary.initialGzipJs` | Raw & Gzip bytes of browser JS in static bootstrap closure. |
| `summary.lazyJs`, `summary.lazyGzipJs` | Raw & Gzip bytes of lazy JS outputs. |
| `summary.totalJs`, `summary.totalGzipJs` | Total uncompressed & Gzip JS bytes. |
| `packages[].name` | npm package owner, including scopes. |
| `packages[].initialBytes` | Emitted input contributions within initial JS outputs. |
| `packages[].lazyBytes` | Emitted input contributions within lazy JS outputs. |
| `packages[].totalBytes` | Combined initial + lazy bytes for this package. |

### Trace JSON (via `--why`)

| Field | Meaning |
| :--- | :--- |
| `target` | Queried target package or module path. |
| `packageName` | Normalized package name if recognized. |
| `found` | Boolean indicating presence in the bundle. |
| `initialBytes`, `lazyBytes`, `totalBytes` | Byte sizes attributed to target. |
| `chains[].output` | The output bundle chunk containing this instance. |
| `chains[].initial` | Whether the output chunk is in initial bootstrap closure. |
| `chains[].path` | Ordered array of module file paths from root to target. |
| `chains[].bytesInChunk` | Contributed bytes inside this specific chunk. |

### Diff JSON (`command: "diff"`)

| Field | Meaning |
| :--- | :--- |
| `schemaVersion` | JSON contract version; currently `"1"`. |
| `toolVersion` | Tool release version; currently `"2.1.0"`. |
| `command` | The string `"diff"`. |
| `summary.before`, `summary.after`, `summary.delta` | Each holds `initialJs`, `lazyJs`, and `totalJs`. |
| `packages[].before`, `packages[].after`, `packages[].delta` | Each holds `initialBytes`, `lazyBytes`, and `totalBytes`. |
| `packages[].status` | `"added"`, `"removed"`, `"changed"`, or `"unchanged"`. |

---

## 🔢 CLI Exit Codes Contract

Integrations and CI scripts can rely on stable, numeric exit codes:

| Code | Name | Meaning |
| :---: | :--- | :--- |
| `0` | Success | Analysis completed successfully, policy satisfied, or report-only check. |
| `1` | Policy Violation | Configured budget threshold was breached or disallowed package detected. |
| `2` | Usage / Config Error | Invalid CLI flags, missing required arguments, or invalid configuration YAML. |
| `3` | Execution Failure | Runtime failure, missing build artifacts, I/O errors, or incomplete workspace report. |

---

## 🛡️ Schema Compatibility & Versioning Guarantees

- **`schemaVersion: "1"`**: Guarantees backwards compatibility for external automation, CI pipelines, and MCP clients.
- **Additive Changes**: New optional fields (such as `findings`, `initialGzipJs`, `gzipBytes`, `drillDown`) may be introduced in minor updates. Automation consumers must accept additive keys without breaking.
- **Breaking Changes**: Modifying existing keys, changing field types, or removing fields will increment `schemaVersion` (e.g., `"2"`).
- **Deterministic Ordering**: Packages, outputs, findings, and trace chains are sorted deterministically with stable secondary tie-breakers across repeated runs.

## 🛠️ Troubleshooting

| Failure | Next check |
| :--- | :--- |
| `no build artifacts found` | Run your build command first to generate `stats.json`. |
| `target package not found` | Verify spelling or check `bundleradar scan` for attributed packages. |
| `baseline file not found` | Check the path provided to `--against` in `bundleradar diff` or `bundleradar gate`. |
| `budget check failed` | Review `violations[]` in JSON or error table in stderr to see which threshold was breached during `bundleradar gate`. |
