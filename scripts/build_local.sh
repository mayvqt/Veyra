#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="${1:-$ROOT_DIR/build/local-run}"
BIN_NAME="veyra"

printf "[build-local] root: %s\n" "$ROOT_DIR"
printf "[build-local] out:  %s\n" "$OUT_DIR"

ENV_BACKUP=""
if [[ -f "$OUT_DIR/.env" ]]; then
  ENV_BACKUP="$(mktemp)"
  cp "$OUT_DIR/.env" "$ENV_BACKUP"
fi

rm -rf "$OUT_DIR"
mkdir -p "$OUT_DIR/internal/http/templates" "$OUT_DIR/web/static" "$OUT_DIR/data"

printf "[build-local] building binary...\n"
(
  cd "$ROOT_DIR"
  GOCACHE="$ROOT_DIR/.cache/go-build" \
  GOMODCACHE="$ROOT_DIR/.cache/gomod" \
  CGO_ENABLED=1 \
  go build -o "$OUT_DIR/$BIN_NAME" ./cmd/server
)

printf "[build-local] copying runtime assets...\n"
cp -R "$ROOT_DIR/internal/http/templates/." "$OUT_DIR/internal/http/templates/"
cp -R "$ROOT_DIR/web/static/." "$OUT_DIR/web/static/"
if [[ -n "$ENV_BACKUP" ]]; then
  cp "$ENV_BACKUP" "$OUT_DIR/.env"
  rm -f "$ENV_BACKUP"
  printf "[build-local] preserved existing .env in bundle\n"
else
  cp "$ROOT_DIR/.env.example" "$OUT_DIR/.env"

  # Generate secure random secrets for local bundle
  SESSION_SECRET="$(openssl rand -hex 32)"
  ENCRYPTION_KEY="$(openssl rand -hex 16)"

  # Replace placeholder values in bundle .env
  sed -i "s|^SESSION_SECRET=.*$|SESSION_SECRET=${SESSION_SECRET}|" "$OUT_DIR/.env"
  sed -i "s|^ENCRYPTION_KEY=.*$|ENCRYPTION_KEY=${ENCRYPTION_KEY}|" "$OUT_DIR/.env"
  printf "[build-local] generated SESSION_SECRET and ENCRYPTION_KEY in bundle .env\n"
fi

cat > "$OUT_DIR/run.sh" <<'RUNEOF'
#!/usr/bin/env bash
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$DIR"

if [[ ! -f .env ]]; then
  echo "Missing .env file. Copy .env.example to .env and set secrets first."
  exit 1
fi

set -a
source ./.env
set +a

export DATABASE_PATH="${DATABASE_PATH:-$DIR/data/veyra.db}"

exec "$DIR/veyra"
RUNEOF

chmod +x "$OUT_DIR/$BIN_NAME" "$OUT_DIR/run.sh"

cat > "$OUT_DIR/README_LOCAL.md" <<'READEOF'
# Veyra Local Run Bundle

## Included
- `veyra` binary
- `internal/http/templates/`
- `web/static/`
- `.env` (copied from `.env.example` and randomized for local secrets)
- `run.sh`

## Run
1. Review `.env` values and adjust service URLs if needed.
2. Start app:
   - `./run.sh`
3. Open:
   - `http://localhost:3767`
READEOF

printf "[build-local] done. Bundle ready at: %s\n" "$OUT_DIR"
