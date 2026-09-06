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

Follow [AGENTS.md](../AGENTS.md). Never commit `.env`, databases, logs, build output, credentials, tokens, or encryption keys.

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

## Local bundle

Run `bash scripts/build_local.sh [output-directory]` to build a local bundle,
then start it with the generated `run.sh`. Rebuilds preserve `.env`, database
files, and unrelated files; failed compilation preserves the previous binary.
New bundles generate secrets and use their own `data/veyra.db`. Existing
`DATABASE_PATH` values are retained, so update older bundles that still point
to the container path `/config/veyra.db` before running on the host.

Image publishing reuses the complete CI workflow, including JavaScript checks,
local bundle regression tests, Go analysis, and the container build.
