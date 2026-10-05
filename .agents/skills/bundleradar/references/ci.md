# Baselines, gates, and CI

## Local baseline & diff loop

```sh
# Diff current build against baseline file or git branch
bundleradar diff dist/my-app/stats.json --against baseline.json -f json
bundleradar diff dist/my-app/stats.json --against main -f json

# Enforce budget limits and regression delta gates
bundleradar gate dist/my-app/stats.json --max-initial 250KB -f json
bundleradar gate dist/my-app/stats.json --max-css 100KB --max-assets 2MB -f json
bundleradar gate dist/my-app/stats.json --against main --max-initial-delta 0B -f json
```

`bundleradar diff` and `bundleradar gate` automatically detect whether `--against` is a local file or a git ref (e.g. `main`, `HEAD~1`). When given a git ref, they create an isolated worktree, build it with `--build-cmd` (default `npm run build`), and compare the emitted bundle.

A negative signed delta is a reduction; a positive delta is growth.

## Reports

`scan`, `diff`, and `gate` support multiple reporter formats:
- `terminal` (default): ANSI-colored human inspection
- `markdown`: Clean tables for GitHub Step Summary and PR bodies
- `github-pr`: Structured PR sticky comment tables with regression attribution
- `json`: Machine-readable output for scripts and agent loops

```sh
bundleradar diff dist/my-app/stats.json --against main -f github-pr -o bundle-report.md
bundleradar gate dist/my-app/stats.json --max-initial 250KB -f markdown
```

## GitHub Action

The repository action (`sonuKumar03/bundleradar@v2.0.0`) runs in GitHub Actions to analyze stats, compare against PR base refs, enforce size and delta budgets, and post PR comments.
