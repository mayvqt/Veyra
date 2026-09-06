#!/bin/sh
set -eu
umask 077

PUID="${PUID:-99}"
PGID="${PGID:-100}"

for ID_VALUE in "$PUID" "$PGID"; do
  case "$ID_VALUE" in
    ''|*[!0-9]*)
      echo "PUID and PGID must be numeric" >&2
      exit 1
      ;;
    *[1-9]*) ;;
    *)
      echo "PUID and PGID must be greater than zero" >&2
      exit 1
      ;;
  esac
done

if [ "$(id -u)" = "0" ]; then
  CURRENT_GID="$(getent group veyra | cut -d: -f3)"
  if [ "$CURRENT_GID" != "$PGID" ]; then
    groupmod -o -g "$PGID" veyra
  fi

  CURRENT_UID="$(id -u veyra)"
  if [ "$CURRENT_UID" != "$PUID" ]; then
    usermod -o -u "$PUID" veyra
  fi

  mkdir -p /config /app
  chown -R veyra:veyra /config /app

  exec gosu veyra "$@"
fi

exec "$@"
