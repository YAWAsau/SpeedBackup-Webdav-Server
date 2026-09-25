#!/usr/bin/env sh
set -eu
HERE=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
BIN="$HERE/speedbackup-server"
ROOT_DIR="${SPEEDBACKUP_SERVER_ROOT:-$HERE/SpeedBackupData}"
LISTEN="${SPEEDBACKUP_SERVER_LISTEN:-0.0.0.0:8765}"
exec "$BIN" serve --root "$ROOT_DIR" --listen "$LISTEN"
