#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BUILD_SCRIPT="$ROOT_DIR/scripts/build_local.sh"
TEST_TMP_ROOT="$ROOT_DIR/.cache/release/tmp"
mkdir -p -- "$TEST_TMP_ROOT"
TEST_ROOT="$(mktemp -d "$TEST_TMP_ROOT/test-build-local.XXXXXX")"
trap 'rm -rf -- "$TEST_ROOT"' EXIT

STUB_BIN_DIR="$TEST_ROOT/stub bin"
mkdir -p -- "$STUB_BIN_DIR"
cat > "$STUB_BIN_DIR/go" <<'GOEOF'
#!/usr/bin/env bash
set -euo pipefail

[[ "${1:-}" == build ]] || exit 2
output=""
while (($#)); do
  if [[ "$1" == -o ]]; then
    output="$2"
    shift 2
  else
    shift
  fi
done
[[ -n "$output" ]]

if [[ "${GO_BUILD_FAIL:-0}" == 1 ]]; then
  exit 17
fi

{
  printf '%s\n' '#!/usr/bin/env bash'
  printf 'printf "stub-build:%%s\\n" %q\n' "${BUILD_ID:-unknown}"
  printf '%s\n' 'printf "DATABASE_PATH=%s\n" "${DATABASE_PATH:-}"'
} > "$output"
chmod 0755 "$output"

if [[ -n "${GO_LOG:-}" ]]; then
  {
    printf 'GOCACHE=%s\n' "$GOCACHE"
    printf 'GOMODCACHE=%s\n' "$GOMODCACHE"
  } > "$GO_LOG"
fi
GOEOF
chmod 0755 "$STUB_BIN_DIR/go"

fail() {
  printf 'test-build-local: %s\n' "$1" >&2
  exit 1
}

assert_file_equals() {
  local expected_file="$1"
  local actual_file="$2"
  cmp -s "$expected_file" "$actual_file" || fail "files differ: $expected_file and $actual_file"
}

bundle="$TEST_ROOT/bundle with spaces"
mkdir -p -- "$bundle/data"
existing_database_path="$TEST_ROOT/existing database/veyra.db"
printf 'APP_NAME=%q\n' 'Existing bundle' > "$bundle/.env"
printf 'DATABASE_PATH=%q\n' "$existing_database_path" >> "$bundle/.env"
printf 'keep this database\n' > "$bundle/data/marker.txt"
printf 'keep this unrelated file\n' > "$bundle/unrelated.txt"
printf 'old binary\n' > "$bundle/veyra"
chmod 0755 "$bundle/veyra"
cp "$bundle/.env" "$TEST_ROOT/existing.env"

custom_gocache="$TEST_ROOT/go build cache"
custom_gomodcache="$TEST_ROOT/go module cache"
custom_go_log="$TEST_ROOT/custom-go.log"
PATH="$STUB_BIN_DIR:$PATH" \
GOCACHE="$custom_gocache" \
GOMODCACHE="$custom_gomodcache" \
GO_LOG="$custom_go_log" \
BUILD_ID=first \
  "$BUILD_SCRIPT" "$bundle" >/dev/null

assert_file_equals "$TEST_ROOT/existing.env" "$bundle/.env"
[[ "$(<"$bundle/data/marker.txt")" == 'keep this database' ]] || fail 'existing data changed on first build'
[[ "$(<"$bundle/unrelated.txt")" == 'keep this unrelated file' ]] || fail 'unrelated file changed on first build'
[[ "$(<"$custom_go_log")" == *"GOCACHE=$custom_gocache"* ]] || fail 'configured GOCACHE was ignored'
[[ "$(<"$custom_go_log")" == *"GOMODCACHE=$custom_gomodcache"* ]] || fail 'configured GOMODCACHE was ignored'
first_run="$("$bundle/run.sh")"
[[ "$first_run" == *'stub-build:first'* ]] || fail 'first binary was not installed'
[[ "$first_run" == *"DATABASE_PATH=$existing_database_path"* ]] || fail 'existing DATABASE_PATH was not respected'

PATH="$STUB_BIN_DIR:$PATH" \
GOCACHE="$custom_gocache" \
GOMODCACHE="$custom_gomodcache" \
GO_LOG="$custom_go_log" \
BUILD_ID=second \
  "$BUILD_SCRIPT" "$bundle" >/dev/null
assert_file_equals "$TEST_ROOT/existing.env" "$bundle/.env"
[[ "$(<"$bundle/data/marker.txt")" == 'keep this database' ]] || fail 'existing data changed on second build'
[[ "$(<"$bundle/unrelated.txt")" == 'keep this unrelated file' ]] || fail 'unrelated file changed on second build'
second_run="$("$bundle/run.sh")"
[[ "$second_run" == *'stub-build:second'* ]] || fail 'second binary was not installed'
cp "$bundle/veyra" "$TEST_ROOT/binary-before-failure"

if PATH="$STUB_BIN_DIR:$PATH" GO_BUILD_FAIL=1 BUILD_ID=failed \
  "$BUILD_SCRIPT" "$bundle" >/dev/null 2>&1; then
  fail 'failed build unexpectedly succeeded'
fi
assert_file_equals "$TEST_ROOT/binary-before-failure" "$bundle/veyra"
if compgen -G "$bundle/.veyra.tmp.*" >/dev/null; then
  fail 'temporary binary was left after failed build'
fi

fresh_bundle="$TEST_ROOT/fresh bundle with spaces"
default_go_log="$TEST_ROOT/default-go.log"
env -u GOCACHE -u GOMODCACHE \
  PATH="$STUB_BIN_DIR:$PATH" \
  GO_LOG="$default_go_log" \
  BUILD_ID=fresh \
  "$BUILD_SCRIPT" "$fresh_bundle" >/dev/null

[[ -d "$fresh_bundle/data" ]] || fail 'fresh bundle data directory was not created'
fresh_run="$("$fresh_bundle/run.sh")"
expected_database_path="$fresh_bundle/data/veyra.db"
[[ "$fresh_run" == *"stub-build:fresh"* ]] || fail 'fresh binary was not installed'
[[ "$fresh_run" == *"DATABASE_PATH=$expected_database_path"* ]] || fail 'fresh bundle did not use its data directory'
[[ ! -e "$fresh_bundle/internal" && ! -e "$fresh_bundle/web" ]] || fail 'embedded assets were copied into the bundle'
[[ "$(<"$default_go_log")" == *"GOCACHE=$ROOT_DIR/.cache/go-build"* ]] || fail 'default GOCACHE was not used'
[[ "$(<"$default_go_log")" == *"GOMODCACHE=$ROOT_DIR/.cache/gomod"* ]] || fail 'default GOMODCACHE was not used'

printf '%s\n' 'test-build-local: passed'
