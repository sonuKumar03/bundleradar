# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- CSS and non-JS asset budget gating: `gate --max-css`/`--max-assets`, YAML
  `budgets.max_css`/`budgets.max_assets`, and MCP `bundle_gate` `max_css`/
  `max_assets` params. CSS is counted across CSS chunks (esbuild) and CSS
  assets (Angular/Vite); new policy rules `MAX_CSS_SIZE` and `MAX_ASSETS_SIZE`.

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

[Unreleased]: https://github.com/sonuKumar03/bundleradar/compare/v2.1.0...HEAD
[2.1.0]: https://github.com/sonuKumar03/bundleradar/compare/v2.0.0...v2.1.0
[2.0.0]: https://github.com/sonuKumar03/bundleradar/compare/v0.6.0...v2.0.0
[0.6.0]: https://github.com/sonuKumar03/bundleradar/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/sonuKumar03/bundleradar/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/sonuKumar03/bundleradar/releases/tag/v0.4.0