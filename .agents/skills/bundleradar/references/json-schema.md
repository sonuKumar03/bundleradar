# JSON Output Fields

BundleRadar CLI JSON output currently has no explicit schema version. The fields below match the current CLI output; tolerate additional fields.

## Scan JSON (`bundleradar scan -f json`)

- Root fields: `metadata`, `entrypoints`, `chunks`, `modules`, `assets`; `gzipEstimated` is optional.
- `metadata`: `bundler`.
- `entrypoints.<name>`: `name`, `initialBytes`, `initialGzipBytes`, `asyncBytes`, `chunkIds`.
- `chunks[]`: `id`, `name`, `path`, `sizeBytes`, `gzipBytes`, optional `gzipEstimated`, `type` (`initial` / `async`), optional `entry`, `moduleIds`.
- `modules[]`: `id`, optional `package` and `version`, `sizeBytes`, `gzipBytes`, `isAppCode`, `chunkIds`; `chunkBytes` and `ingressPaths` may be present when available.
- `assets[]`: `path`, `sizeBytes`, `gzipBytes`, optional `gzipEstimated`, `mimeType`.

## Diff JSON (`bundleradar diff -f json`)

- `summary`: `baseInitialBytes`, `headInitialBytes`, `initialDeltaBytes`, `baseLazyBytes`, `headLazyBytes`, `lazyDeltaBytes`, `baseTotalBytes`, `headTotalBytes`, `totalDeltaBytes`.
- `entrypoints.<name>`: `name`, `initialDeltaBytes`, `initialGzipDeltaBytes`, `asyncDeltaBytes`.
- `packages[]` and optional `unchangedPackages[]`: `name`, `deltaBytes`, `gzipDeltaBytes`, `baseBytes`, `currBytes`, `status`, with optional `chunkNames` and `importPath`.
- `addedChunks[]`, `removedChunks[]`, `microDriftBytes`, `attributions[]`.

## Gate JSON (`bundleradar gate -f json`)

- `passed`: boolean.
- `violations[]` and `warnings[]`: evaluated policy results.
