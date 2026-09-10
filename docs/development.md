# Development

Start with the [development task index](development/README.md). It links to the
code map, architecture, data, validation, operations, and documentation guides.

For a local server, use the Go version declared in `go.mod`, a C compiler for
CGO SQLite, and Node for the dependency-free JavaScript checks. No `npm install`
is needed. Assets are embedded in the Go binary, so rebuild after changing
templates, JavaScript, or CSS.

```bash
set -a; source .env; set +a
DATABASE_PATH=./veyra.db go run ./cmd/server
```

Never commit `.env`, databases, logs, build output, credentials, tokens, or encryption keys.

## Local bundle

Run `bash scripts/build_local.sh [output-directory]` to build a local bundle,
then start it with the generated `run.sh`. Rebuilds preserve `.env`, database
files, and unrelated files; failed compilation preserves the previous binary.
New bundles generate secrets and use their own `data/veyra.db`. Existing
`DATABASE_PATH` values are retained, so update older bundles that still point
to the container path `/config/veyra.db` before running on the host.

See [Validation](development/validation.md) before submitting changes.
