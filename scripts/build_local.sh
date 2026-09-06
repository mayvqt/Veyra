#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="${1:-$ROOT_DIR/build/local-run}"
BIN_NAME="veyra"

if [[ "$OUT_DIR" != /* ]]; then
  OUT_DIR="$PWD/$OUT_DIR"
fi
mkdir -p -- "$OUT_DIR"
OUT_DIR="$(cd -- "$OUT_DIR" && pwd -P)"

printf "[build-local] root: %s\n" "$ROOT_DIR"
printf "[build-local] out:  %s\n" "$OUT_DIR"

mkdir -p -- "$OUT_DIR/data"

TEMP_BIN=""
TEMP_ENV=""
cleanup() {
  if [[ -n "$TEMP_BIN" && -e "$TEMP_BIN" ]]; then
    rm -f -- "$TEMP_BIN"
  fi
  if [[ -n "$TEMP_ENV" && -e "$TEMP_ENV" ]]; then
    rm -f -- "$TEMP_ENV"
  fi
}
trap cleanup EXIT

TEMP_BIN="$(mktemp "$OUT_DIR/.${BIN_NAME}.tmp.XXXXXX")"

printf "[build-local] building binary...\n"
(
  cd "$ROOT_DIR"
  GOCACHE="${GOCACHE:-$ROOT_DIR/.cache/go-build}" \
  GOMODCACHE="${GOMODCACHE:-$ROOT_DIR/.cache/gomod}" \
  CGO_ENABLED=1 \
  go build -o "$TEMP_BIN" ./cmd/server
)
chmod 0755 -- "$TEMP_BIN"
mv -f -- "$TEMP_BIN" "$OUT_DIR/$BIN_NAME"
TEMP_BIN=""

if [[ -f "$OUT_DIR/.env" ]]; then
  printf "[build-local] preserved existing .env in bundle\n"
else
  cp "$ROOT_DIR/.env.example" "$OUT_DIR/.env"

  # Generate secure random secrets for local bundle
  SESSION_SECRET="$(openssl rand -hex 32)"
  ENCRYPTION_KEY="$(openssl rand -hex 16)"

  # Replace placeholder values in bundle .env
  sed -i "s|^SESSION_SECRET=.*$|SESSION_SECRET=${SESSION_SECRET}|" "$OUT_DIR/.env"
  sed -i "s|^ENCRYPTION_KEY=.*$|ENCRYPTION_KEY=${ENCRYPTION_KEY}|" "$OUT_DIR/.env"

  # Keep fresh local bundles self-contained while allowing existing .env files
  # to choose an explicit database location.
  LOCAL_DATABASE_PATH="$OUT_DIR/data/veyra.db"
  TEMP_ENV="$(mktemp "$OUT_DIR/.env.tmp.XXXXXX")"
  while IFS= read -r line || [[ -n "$line" ]]; do
    if [[ "$line" == DATABASE_PATH=* ]]; then
      printf 'DATABASE_PATH=%q\n' "$LOCAL_DATABASE_PATH"
    else
      printf '%s\n' "$line"
    fi
  done < "$OUT_DIR/.env" > "$TEMP_ENV"
  mv -f -- "$TEMP_ENV" "$OUT_DIR/.env"
  TEMP_ENV=""
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
- embedded templates and static assets
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
