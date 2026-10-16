# JSON Output Fields

BundleRadar CLI JSON output currently has no explicit schema version. The fields below match the current CLI output; tolerate additional fields.

## Scan JSON (`bundleradar scan -f json`)

- Root fields: `metadata`, `entrypoints`, `chunks`, `modules`, `assets`; `gzipEstimated` and `warnings` are optional. `warnings` lists failed internal consistency checks; treat the numbers as unreliable when it is present and say so in the report.
- `metadata`: `bundler`.
- `entrypoints.<name>`: `name`, `initialBytes`, `initialGzipBytes`, `asyncBytes`, `chunkIds`. Byte totals are JavaScript only; stylesheets are reported through `assets` and `--max-css`.
- `chunks[]`: `id`, `name`, `path`, `sizeBytes`, `gzipBytes`, optional `gzipEstimated`, `type` (`initial` / `async`), optional `entry`, `moduleIds`.
- `modules[]`: `id`, optional `package` and `version`, `sizeBytes`, `gzipBytes`, `isAppCode`, `chunkIds`; `chunkBytes` and `ingressPaths` may be present when available.
- `assets[]`: `path`, `sizeBytes`, `gzipBytes`, optional `gzipEstimated`, `mimeType`.

## Diff JSON (`bundleradar diff -f json`)

- `summary`: `baseInitialBytes`, `headInitialBytes`, `initialDeltaBytes`, `baseLazyBytes`, `headLazyBytes`, `lazyDeltaBytes`, `baseTotalBytes`, `headTotalBytes`, `totalDeltaBytes`.
- `entrypoints.<name>`: `name`, `initialDeltaBytes`, `initialGzipDeltaBytes`, `asyncDeltaBytes`.
- `packages[]`: `name`, `deltaBytes`, `gzipDeltaBytes`, `baseBytes`, `currBytes`, `status`, with optional `chunkNames`, `importPath`, and `initialDeltaBytes`. `status` is `ADDED`, `ELIMINATED`, `REDUCED`, `REGRESSED`, or, when the total is unchanged but the package moved into or out of initial chunks, `PROMOTED` or `DEFERRED`. Rank by the larger of `deltaBytes` and `initialDeltaBytes`.
- `firstParty[]`: the same fields per workspace project (`apps/<name>`, `libs/<name>`).
- `unchangedCount` always; `unchangedPackages[]` only when requested (`include_unchanged` over MCP).
- `addedChunks[]`, `removedChunks[]` (no module lists), `changedChunks[]` (same chunk under a new hashed name), `microDriftBytes`, `attributions[]`.

## Why JSON (`bundleradar scan --why <pkg> -f json`)

- `target`, `packageName`, `found`, `initialBytes`, `lazyBytes`, `totalBytes`, `chains[]` (`output`, `initial`, `path`, `bytesInChunk`).
- `retainedInitialBytes` and `importers[]` (`module`, `firstParty`, `savingBytes`) when the stats expose import edges (Angular and esbuild). `savingBytes` is the initial JS saved by removing only that import; 0 means another importer keeps the package in.

## Gate JSON (`bundleradar gate -f json`)

- `passed`: boolean.
- `violations[]` and `warnings[]`: evaluated policy results.
