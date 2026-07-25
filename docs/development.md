# Development

```bash
set -a; source .env; set +a
DATABASE_PATH=./veyra.db go run ./cmd/server

gofmt -w <touched-go-files>
go test ./...
go vet ./...
go test -race ./...
git diff --check
```

Follow [AGENTS.md](../AGENTS.md). Never commit `.env`, databases, logs, build output, credentials, tokens, or encryption keys.
