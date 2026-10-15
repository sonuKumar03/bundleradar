# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- Import graph and ingress paths now follow CommonJS `require()` edges, so
  `--why` retained size and importer savings are correct for CJS packages
  (e.g. moment-timezone, exceljs).
- Diff ranks packages by the larger of their total and initial-bundle change,
  so a package moving into the initial bundle is not buried under noise.

## [2.5.0] - 2026-10-15

### Fixed

- Angular parser: derive initial roots from the metafile when `index.html`
  matches no emitted file (deploy-URL placeholders, CDN references), instead
  of reporting 0 initial JS.
- Gate: fail with `INITIAL_SIZE_UNRESOLVED` when an initial budget is set but
  no initial roots were detected, instead of passing silently.
- Diff: report a package moving between lazy and initial chunks
  (`PROMOTED`/`DEFERRED`, `initialDeltaBytes`) instead of `UNCHANGED`; group
  first-party changes by workspace project; pair content-hashed chunks into
  `changedChunks`; omit module lists from added/removed chunks; list unchanged
  packages only on request (`unchangedCount` is always reported).
- `--why` and diff attributions prefer static import chains over shorter
  chains through dynamic imports.
- esbuild parser: set module versions so duplicate-version detection works.
- Nx discovery no longer assumes a `browser/` subdirectory for unbuilt apps.
- MCP gate loads `.bundleradar.yml` relative to the stats file, not only the
  server's working directory.

### Added

- `--why` reports `retainedInitialBytes` and static `importers` with the
  initial JS each import would save (esbuild and Angular stats).
- MCP: `entry` on `bundle_scan`/`bundle_diff`/`bundle_gate`; `dist` and
  `bundler` on `bundle_diff`/`bundle_gate`; `include_unchanged` on `bundle_diff`.
- Config: `scan.bundler`/`scan.dist`/`scan.entry`, `budgets.max_lazy`,
  `budgets.max_total_delta`, `rules.detect_duplicate_packages`.

## [2.4.1] - 2026-10-14

### Changed

- Docs: trim README by removing verbose sections covered in the docs site
  (collapsed command reference, CI inputs table, installation binaries list,
  and contributing section).
- Docs: update SKILL.md for universal bundler support.
- Docs: add CLI and GitHub Action cheatsheet (`docs/cheatsheet.md`).

## [2.4.0] - 2026-10-14

### Added

- Esbuild parser: `--why` import chain tracing via shared BFS ingress path
  builder extracted from the Angular parser.
- Webpack parser: `--why` import chain tracing using `issuerPath` fields,
  module deduplication across chunks, and top-level asset parsing.
- Vite parser: process chunk assets, build partial ingress paths from
  chunk-level imports via BFS, and extract installed package versions.
- Persistent build history: JSON file persistence for build checkpoints
  across server restarts (`--history-file` flag, default
  `.bundleradar-history.json`).
- CLI `history` subcommand: `history save`, `history list`, `history show`
  for recording and inspecting build history without the server.
- Studio: Trends tab with inline SVG line chart showing Initial JS, Total JS,
  and Gzip estimates across checkpoints.
- GitHub Action: `history` and `history-file` inputs for recording build
  history checkpoints in CI.

### Fixed

- Gzip estimation: change binary file ratio from 0.0 to 1.0 so images and
  other pre-compressed files report realistic gzip sizes.

### Changed

- Docs site and README rebranded from Angular-only to universal bundler
  support (Angular, esbuild, Vite, Webpack).
- GitHub Action renamed from "Angular Bundle Analyzer" to "Universal Bundle
  Analyzer".

## [2.3.1] - 2026-10-13

### Fixed

- Action: append gate policy violations (e.g. `MAX_CSS_SIZE`,
  `MAX_ASSETS_SIZE`) to the markdown report file when `output` is set,
  so downstream assertions can find them in the report instead of only
  in terminal output.

## [2.3.0] - 2026-10-13

### Added

- Studio treemap: squarified layout algorithm for near-square tile aspect
  ratios, replacing the previous slice-and-dice layout. Click any package
  tile to drill down into its constituent modules; right-click or use the
  breadcrumb to navigate back. Raw/Gzip size toggle re-layouts the treemap.
- Bundle API: expose module-level data (`modules` array) alongside the
  existing `topPackages` for treemap drill-down and future tooling.

### Changed

- Extract `ApplyOverrides` helper to deduplicate repeated policy override
  parsing in the gate CLI and MCP handler.
- Extract `aggregatePackages` in diff engine to consolidate redundant
  per-bundle iteration into a single pass.
- Use `c.Context()` instead of `context.Background()` in CLI commands to
  propagate cobra cancellation signals.

### Fixed

- Esbuild parser: deduplicate modules that appear in multiple output chunks,
  preventing inflated size attribution.
- `ResolveRef`: fail explicitly on unknown git refs instead of silently
  passing through an unresolved string.
- Gzip measurement: stream from file handle with pooled buffers instead of
  materializing the full file in memory.
- Vite parser: cache `fileSize` results to avoid redundant `os.Stat` syscalls.
- CORS: restrict `Access-Control-Allow-Origin` to the bound host for
  non-localhost listeners.
- Worktree: cap build output capture at 64 KB to prevent unbounded memory
  usage from verbose build processes.

## [2.2.0] - 2026-10-05

### Added

- CSS and non-JS asset budget gating: `gate --max-css`/`--max-assets`, YAML
  `budgets.max_css`/`budgets.max_assets`, MCP `bundle_gate` `max_css`/
  `max_assets` params, and GitHub Action `max-css`/`max-assets` inputs. CSS is
  counted across CSS chunks (esbuild) and CSS assets (Angular/Vite); new policy
  rules `MAX_CSS_SIZE` and `MAX_ASSETS_SIZE`.

### Fixed

- Angular: classify initial files from emitted HTML, trace lazy import ingress
  chains, and align JavaScript size attribution across entrypoints.
- MCP: fail closed on gate input errors, align baseline contracts, and enforce
  lazy/total-delta budgets.
- Action: apply config policies, auto-detect stats safely, select artifact
  baselines by branch run, compare multi-app targets to the base ref, resolve
  Nx project metadata, and report incomplete workspace scans.
- Studio: label the size-threshold indicator as a heuristic.

## [2.1.0] - 2026-09-29

### Changed

- Detect bundler formats structurally (required JSON keys) instead of by
  substring sniffing, so values that merely mention a keyword can no longer
  route a file to the wrong parser.
- Reporters: `--why` import tracing now lives in a reusable `WhyReporter`
  (terminal/JSON) shared by CLI and server consumers.
- Bundle index lifecycle: `AddChunk`/`AddModule` rebuild the lookup index
  after JSON deserialization instead of panicking or reading stale positions.

### Fixed

- Terminal gzip output no longer shows `~` when gzip was measured from the
  emitted file; an explicit note appears when sizes are ratio-estimated.
- Worktree baseline builds (`--against <git-ref>`) now run in the directory
  owning `package.json`, supporting monorepos, with `--build-dir` override
  and actionable hints when stats are gitignored.
- Worktree `--no-build` reports a clear hint instead of a bare "no such file".

## [2.0.0] - 2026-08

### Added

- Real build verification across Angular/Nx, Vite, and webpack fixtures with
  invariant tests and a committed-stats drift guard.
- Streaming stats decode via `json.Decoder` (roughly halves peak memory on
  multi-hundred-MB metafiles) with context-cancellable parsing.
- Real gzip measurement of emitted chunk/asset files, with per-MIME ratio
  fallbacks and a `gzipEstimated` flag in the JSON output.
- Shared package-movement status semantics between the CLI, web UI, and MCP
  (`ELIMINATED`/`REDUCED`/`REGRESSED`/`ADDED`/`UNCHANGED`).
- Deterministic parser output (sorted metafile iteration) for stable diffs.
- Typed error classification so exit codes (0/1/2/3) never depend on error
  message strings.

### Changed

- Release binaries inject the version via `-ldflags` instead of a hard-coded
  constant.

## [0.6.0] - 2026-07

### Added

- Live web UI Studio with SSE build streaming, checkpoints history, and
  side-by-side diff view.
- `ui` command and `scan --ui` launch flag.
- `scan --why` package import tracing and `--top` package listing.

## [0.5.0] - 2026-06

### Added

- Git worktree baseline comparison for `--against <git-ref>`.
- Install/uninstall shell scripts with GitHub release downloads.
- Nx workspace auto-discovery and `workspace scan` aggregate reporting.

## [0.4.0] - 2026-05

### Added

- First universal bundle analyzer release: Angular esbuild metafile parsing,
  initial vs async byte attribution, package contribution breakdown, and
  CI budget gating.
- GitHub Action for CI bundle checks with baseline artifacts.

[Unreleased]: https://github.com/sonuKumar03/bundleradar/compare/v2.5.0...HEAD
[2.5.0]: https://github.com/sonuKumar03/bundleradar/compare/v2.4.1...v2.5.0
[2.4.1]: https://github.com/sonuKumar03/bundleradar/compare/v2.4.0...v2.4.1
[2.4.0]: https://github.com/sonuKumar03/bundleradar/compare/v2.3.1...v2.4.0
[2.3.1]: https://github.com/sonuKumar03/bundleradar/compare/v2.3.0...v2.3.1
[2.3.0]: https://github.com/sonuKumar03/bundleradar/compare/v2.2.0...v2.3.0
[2.2.0]: https://github.com/sonuKumar03/bundleradar/compare/v2.1.0...v2.2.0
[2.1.0]: https://github.com/sonuKumar03/bundleradar/compare/v2.0.0...v2.1.0
[2.0.0]: https://github.com/sonuKumar03/bundleradar/compare/v0.6.0...v2.0.0
[0.6.0]: https://github.com/sonuKumar03/bundleradar/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/sonuKumar03/bundleradar/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/sonuKumar03/bundleradar/releases/tag/v0.4.0