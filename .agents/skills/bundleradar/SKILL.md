---
name: bundleradar
description: >
  Use this skill when the user wants to understand, compare, or reduce the
  JavaScript bundle size of an application built with Angular, esbuild, Vite,
  or Webpack. Use it to identify initial or lazy bundle contributors, trace
  why a package or source file is bundled, diagnose bundle regressions,
  measure an optimization against a baseline, enforce bundle budgets, or
  inspect costs across Nx applications. Do not use it for runtime CPU,
  memory, Lighthouse, Web Vitals, or network profiling.
compatibility: >
  Requires the bundleradar CLI and a bundler stats file (Angular stats.json,
  esbuild meta.json, Vite manifest.json, or Webpack stats.json). Node and Nx
  are required only to build or inspect an Nx workspace. Git is required only
  for git-ref baseline workflows.
---

# BundleRadar

Use BundleRadar as the reasoning loop for JavaScript bundle work (Angular, esbuild, Vite, Webpack). Prefer exact bytes and measured deltas over generic optimization advice.

## Route the request

| User intent | First operation |
| --- | --- |
| What is large? | `scan` |
| Why is X bundled? | `scan --why <pkg>` |
| What should I optimize? | `scan` (review top packages) |
| Optimize this app | `scan` → edit → rebuild → `diff` → `gate` loop |
| Did this PR regress? | `diff --against <ref>` |
| Prevent regressions | `gate` |
| Compare Nx applications | `workspace scan` |

## Choose execution

1. Use matching `bundle_scan`, `bundle_diff`, `bundle_gate`, or `workspace_summary` MCP tools when available.
2. Otherwise verify `command -v bundleradar` and run CLI commands with `-f json`.
3. If missing, stop and offer the supported remote install command:

```sh
curl -fsSL https://raw.githubusercontent.com/sonuKumar03/bundleradar/master/install.sh | sh
```

Never assume the BundleRadar repository or a local `./install.sh` exists. MCP and CLI follow the same decisions; only transport differs.

## Optimize safely

1. Build production stats when artifacts are missing or stale, normally with `ng build --configuration production --stats-json`.
2. Capture the pre-edit baseline.
3. Rank candidates with `scan`.
4. Confirm one candidate with `scan --why <pkg>`, then inspect its actual application source usage. Use `retainedInitialBytes` for the realistic saving and `importers[].savingBytes` to see which import to cut; if every importer saves 0, all of them must go.
5. Make the smallest authorized application change; preserve unrelated work.
6. Rebuild production stats and run relevant application tests.
7. Measure against the baseline with `diff` and run budget enforcement with `gate`.
8. Keep the change only when measured evidence supports it. Otherwise report the result without deleting unrelated or pre-existing work.

Treat `savingsBytes` and `savingsGzipBytes` as estimates until a production rebuild and measure confirm the delta. Static bundle evidence cannot establish LCP, INP, memory, CPU, or other runtime improvements.

## Report

Include:

- baseline and current initial bytes;
- signed initial-byte delta;
- affected packages or sources;
- edited files;
- tests, rebuilds, and gates run;
- remaining uncertainty.

Do not turn byte reductions into runtime-performance claims.

## Load detail only when needed

- Read [references/cli.md](references/cli.md) only for CLI fallback, installation, discovery, flags, or failures.
- Read [references/json-schema.md](references/json-schema.md) only when manually interpreting JSON or writing automation.
- Read [references/nx.md](references/nx.md) only for Nx or multi-app work.
- Read [references/ci.md](references/ci.md) only for baselines, budget gates, GitHub Actions, or PR reporting.
