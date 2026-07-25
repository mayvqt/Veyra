#!/bin/sh
set -eu

PUID="${PUID:-99}"
PGID="${PGID:-100}"

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
