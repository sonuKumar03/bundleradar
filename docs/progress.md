# bundleradar progress

Last updated: 2026-09-29

Update this document when a milestone changes. Mark work complete only after
implementation and verification; record remaining limitations separately.

## Current status

BundleRadar v2 is officially released (`v2.0.0`) with a clean-slate hexagonal architecture centered on 5 orthogonal verbs: `scan`, `diff`, `gate`, `workspace`, and `mcp`. All CI pipelines, cross-platform build matrices, Go microbenchmarks, and GitHub Actions integrations are fully verified and passing. Latest release is `v2.1.0`, which adds the live web UI Studio and ships hardened parsers, MCP contracts, and release automation.

## Post-v2.1.0 (unreleased)

- [x] CSS and non-JS asset budget gating: `gate --max-css`/`--max-assets` flags, YAML `budgets.max_css`/`budgets.max_assets`, MCP `bundle_gate` `max_css`/`max_assets` params, and `MAX_CSS_SIZE`/`MAX_ASSETS_SIZE` policy rules. CSS is counted across CSS chunks (esbuild) and CSS assets (Angular/Vite); asset budgets cover images/fonts/media only.

## v2.1.0

- [x] `bundleradar ui` command (and `scan --ui`) serving a live web Studio with embedded static assets (`go:embed`), real-time file watcher with SSE live stream, build checkpoints API, and `/api/bundle` ingress attribution.
- [x] 60fps Canvas treemap, search table, side-by-side diff view, and live-sync UI with diagnostic scorecards.
- [x] Real Vite and webpack fixture apps with parser fixes surfaced by real builds; monorepo-aware worktree baseline builds (`--build-dir` override).
- [x] Structural bundler format detection (required JSON keys) instead of substring sniffing; typed error exit codes.
- [x] Honest gzip: measured vs. ratio-estimated gzip distinguished in terminal/JSON output.
- [x] Multi-arch GoReleaser builds with SBOM + cosign signing; `uninstall.sh` and go.mod tidy.
- [x] Post-release hardening sprint merged to `master` (contract alignment for MCP baselines, Nx project metadata discovery, Angular lazy-ingress trails, multi-app Action baselines, Studio size-threshold heuristic labeling).

## Completed

- [x] Clean-slate v2 architecture designed and implemented across `internal/core`, `internal/adapters`, and `cmd/`.
- [x] Streamlined CLI verbs: `scan`, `diff`, `gate`, `workspace`, and `mcp`.
- [x] Universal Bundle AST in `internal/core/bundle.go` with zero external dependencies.
- [x] Pluggable parsers registry in `internal/adapters/parsers/` supporting Angular 17+ esbuild, Esbuild, Vite, and Webpack.
- [x] Angular 17+ esbuild BFS reachability traversal for initial vs. lazy chunks and directed shortest ingress import path resolution (`internal/adapters/parsers/angular.go`).
- [x] Universal Diff & Source Attribution Engine in `internal/core/diff/` with `DiffSummary`, chunk attribution, import paths, and micro-drift bucketing.
- [x] Rich GitHub PR Markdown & Terminal diff reporters with Initial/Lazy/Total JS comparison tables, ingress chains, and collapsible unchanged packages.
- [x] Policy & Budget Evaluation Gate in `internal/core/policy/`.
- [x] Multi-format reporters in `internal/adapters/reporters/` (Terminal, Markdown, GitHub PR, JSON).
- [x] Monorepo & multi-app workspace discovery in `internal/adapters/workspaces/`.
- [x] Public Go SDK in `pkg/bundleradar/`.
- [x] Modernized MCP server (`bundleradar mcp`) with updated tools (`bundle_scan`, `bundle_diff`, `bundle_gate`, `workspace_summary`) and rules resource.
- [x] Full GitHub Action (`action.yml`) modernization and automated test suite.
- [x] Documentation & schema contract test suite passing.
- [x] Cross-platform build and release automation for Linux, macOS, and Windows.
- [x] Official `v2.0.0` release published on GitHub Releases with floating major tag `v2`.

## Verification recorded

The latest implementation checks on 2026-09-24 passed:

| Check | Result |
| --- | --- |
| `rtk go test ./...` | 71 tests passed across 11 packages (zero failures, zero regressions) |
| `rtk go vet ./...` | Passed cleanly |
| `rtk go build -o bundleradar .` | Passed |
| Real Angular 17+ app verification | Tested against real Nx workspace builds (`testdata/nx-workspace/apps/portal` and `bundlecheck-nx-test`) |
| Directed BFS ingress path tracing | Verified exact import trail: `apps/portal/src/main.ts → apps/portal/src/app/app.component.ts → node_modules/three/build/three.module.js` |
| Rich PR Diff Markdown & Terminal output | Verified category summary table (Initial/Lazy/Total JS), chunk attribution, and collapsible unchanged packages |
| MCP tool suite | Verified `bundle_scan`, `bundle_diff`, `bundle_gate`, and `workspace_summary` |
