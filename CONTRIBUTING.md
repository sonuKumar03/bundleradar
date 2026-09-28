# Contributing to bundleradar

Thanks for wanting to help. bundleradar is a Go CLI + SDK for JavaScript bundle
analysis, built around real-build verification and exact bytes.

## Development setup

Requirements: Go 1.27+, and Node 22 for the real bundler fixtures.

```bash
# Build the CLI
go build -o bundleradar .

# Run the full test suite (unit + integration + contract tests)
go test ./...

# Race detector on changed packages
go test -race -count=1 ./internal/...

# Lint (golangci-lint is used in CI)
go vet ./...
golangci-lint run
```

The CI benchmark job runs `go test -bench=. -benchmem -run=^# ./...`; keep
parser benchmarks representative of real (large) metafiles.

## Design docs

Substantial design decisions live in [`docs/superpowers/specs`](docs/superpowers/specs)
and progress notes in [`docs/progress.md`](docs/progress.md). If you are
changing behavior that others depend on (CLI output, JSON schema, exit codes,
GitHub Action inputs), read and update the relevant design/spec docs, and keep
the documentation contract tests (`docs_contract_test.go`) passing.

## Working with real builds

The e2e job builds real Angular/Nx, Vite, and webpack fixtures in
`testdata/`. New parser behavior should be verified against a real build, not
only synthetic fixtures:

```bash
cd testdata/nx-workspace && npm ci && npm run build
cd ../../ && go test -count=1 ./...
```

Tests that require built artifacts skip when the output is absent; they **fail**
under `BUNDLECHECK_REQUIRE_E2E=1` (set in CI) so the e2e job cannot silently
skip.

## Commit conventions

- One logical change per commit; message format follows the repo's
  conventional-commit style (`feat:`, `fix:`, `refactor:`, `test:`, `docs:`).
- PRs are **squash-merged** with the PR title as the commit subject; write the
  PR body so it reads as a commit message.
- Do not create merge commits; keep `master` linear via squash.

## Release process

Releases are cut by tagging `vX.Y.Z` matching the `VERSION` file; CI verifies
consistency, then GoReleaser builds the multi-arch artifacts, generates an SBOM,
signs them with cosign (keyless), and publishes the GitHub release. See
`.goreleaser.yml` and `.github/workflows/release.yml`. Keep the release asset
naming contract in sync with `install.sh`.

## Reporting issues

- Bundle size/attribution inaccuracies: include the bundler, its version, and
  the stats/metafile you scanned.
- CI failures: link the workflow run.

## Code of conduct

Be constructive. This is a small, friendly project.