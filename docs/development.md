# Development

Use the Go version declared in `go.mod`, a C compiler for CGO SQLite, and Node/npm
for dependency-free JavaScript checks. Assets are embedded in the Go binary;
rebuild after changing templates, JavaScript, or CSS. No npm install is needed.

```bash
set -a; source .env; set +a
DATABASE_PATH=./veyra.db go run ./cmd/server

gofmt -w <touched-go-files>
go test ./...
go vet ./...
go test -race ./...
npm run lint
npm test
bash scripts/test-build-local.sh
git diff --check
```

Never commit `.env`, databases, logs, build output, credentials, tokens, or encryption keys.

## Isolated validation

Go tests create disposable SQLite databases with `t.TempDir()` or private
in-memory databases and synthetic upstream HTTP fixtures. They do not need a
running media server, Docker, or a shared database. Run focused package tests
while editing; run the commands above plus `CGO_ENABLED=1 go build -trimpath -o
./build/veyra ./cmd/server` for a release pass. CI also runs Staticcheck,
govulncheck, and a Docker build; use existing installed tools locally and ask
before installing missing tools or downloading dependencies.

If the default cache or temporary directory is unavailable, create an ignored
directory under `.cache/` and set `GOCACHE`, `GOMODCACHE`, and `TMPDIR` to absolute
paths there. Reuse them across checks rather than creating fresh caches per run.

No browser wrapper is checked in. Use available shared browser tooling against
a disposable local instance, with synthetic data, for desktop/mobile and
interaction checks. Record any unverified login, admin, integration, or UI states.

## Architecture and UI

Read [Frontend and UI](architecture.md#frontend-and-ui) before changing templates,
CSS, or browser JavaScript. Keep trusted decisions on the server, preserve
server-rendered behavior, and reuse the existing CSS and feature-module boundaries.

## Local browser visual checks

Build or run a disposable local instance with synthetic data, then check the first
meaningful screen at desktop and mobile widths. Verify page identity, useful
rendered content, the absence of framework/runtime overlays and relevant console
errors, and at least one primary interaction. Inspect screenshots for clipping,
overlap, accidental wrapping, unreadable text, scroll traps, and broken responsive
states. Store temporary screenshots and browser scripts outside the repository.

## Data and schema

SQLite access and migrations belong to `internal/store/`. Every schema change needs
a new forward-only migration and coverage proving both a fresh database and an
upgrade from the previous schema. Never edit or delete a migration that may have
run in an existing installation. Use disposable databases for checks and do not
point tests at a user's application database.

## Validation and coordination

Run focused checks while editing and expand to the full validation list above once
for the reviewed batch. Give one agent ownership of each changed path, avoid
duplicating passing checks, and review the stable combined diff before committing.
Browser screenshots validate presentation; they do not replace programmatic tests.

## Refactor checks

For refactors, concurrency, or performance work, add or retain tests for the
behavioral contract before changing structure. Run focused tests with the race
detector where shared state or goroutine lifecycles change, and benchmark only the
specific hot path being claimed. Prefer measured fixes over speculative caching or
parallelism.

## Documentation conventions

Update the canonical document for every user-visible or operator-visible behavior
changed in the same batch. Keep `README.md` as a short navigation and quick-start
page; put durable detail in the focused files under `docs/`. Document verified
behavior rather than plans, avoid duplicating configuration tables, and use
relative links to canonical repository sources.

## Local bundle

Run `bash scripts/build_local.sh [output-directory]` to build a local bundle,
then start it with the generated `run.sh`. Rebuilds preserve `.env`, database
files, and unrelated files; failed compilation preserves the previous binary.
New bundles generate secrets and use their own `data/veyra.db`. Existing
`DATABASE_PATH` values are retained, so update older bundles that still point
to the container path `/config/veyra.db` before running on the host.

Image publishing reuses the complete CI workflow, including JavaScript checks,
local bundle regression tests, Go analysis, and the container build.
