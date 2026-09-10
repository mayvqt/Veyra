# Validation

Use focused checks while developing:

| Change | Focused check |
| --- | --- |
| Application lifecycle or backup | `go test ./internal/app` |
| Authentication | `go test ./internal/auth` |
| Routes, handlers, templates, or middleware | `go test ./internal/http/...` |
| Media server, Seerr, or Arr client | `go test ./internal/integrations/mediaserver`, `go test ./internal/integrations/seerr`, or `go test ./internal/integrations/arr` |
| SQLite or migrations | `go test ./internal/store` |
| Configuration, encryption, or logging | `go test ./internal/config ./internal/security ./internal/logging` |
| Browser JavaScript | `npm run lint && npm test` |
| Embedded assets | `go test .` |
| Entrypoint or local bundle | `sh scripts/test-entrypoint.sh` or `bash scripts/test-build-local.sh` |
| Release binary | `CGO_ENABLED=1 go build -trimpath -o /tmp/veyra ./cmd/server` |

Format touched Go files with `gofmt -w`. No npm dependencies are installed.
Performance or refactor claims need representative before/after measurements and
a regression threshold.

For UI work, rebuild the binary and use a disposable database with synthetic
upstream fixtures. At desktop and narrow mobile widths, check keyboard navigation,
visible focus, labels, contrast, and the loading, empty, stale/partial, validation
error, integration error, signed-out, member-denied, and administrator states
touched by the change. Cover setup and login separately because they do not use
the authenticated shell.

Use the Go version in `go.mod`, a working C compiler/SQLite headers, and Node 24.
Do not install missing tools or download dependencies without approval. Reuse
local caches only when `go.sum`, toolchain, OS/architecture, CGO environment,
and installed module tree match the revision; otherwise validate in a clean
locked environment or stop.

The final gate is the full GitHub Actions `CI` workflow in
`.github/workflows/ci.yml` on the exact revision. It covers formatting,
whitespace, JavaScript, entrypoint and bundle safety, Go tests, vet, race
detection, pinned Staticcheck and govulncheck versions, the CGO build, and Docker.
