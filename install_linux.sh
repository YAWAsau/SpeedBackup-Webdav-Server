#!/bin/sh
# Install a local Debian package and choose its boot policy.
set -eu
mode=keep
if [ "${1:-}" = --autostart ]; then mode=${2:-}; shift 2; fi
[ "$#" -eq 1 ] || { echo 'usage: sudo sh install_linux.sh [--autostart on|off|keep] PACKAGE.deb' >&2; exit 2; }
case "$mode" in on|off|keep) ;; *) echo 'autostart must be on, off or keep' >&2; exit 2;; esac
[ "$(id -u)" -eq 0 ] || { echo 'Run this installer with sudo.' >&2; exit 1; }
package=$(realpath -- "$1")
[ "$(dpkg-deb -f "$package" Package)" = speedbackup-server ] || { echo 'Unexpected package' >&2; exit 1; }
if [ "$mode" = keep ] && [ -t 0 ]; then
  printf 'Start at boot? [y/n/Enter keeps current; new installation defaults to yes]: '
  read -r answer
  case "$answer" in y|Y) mode=on;; n|N) mode=off;; '') ;; *) echo 'Enter y or n' >&2; exit 2;; esac
fi
dpkg -i "$package"
case "$mode" in
  on) /usr/bin/speedbackup-server service autostart --enabled=true;;
  off) /usr/bin/speedbackup-server service autostart --enabled=false;;
esac
# Apply the user's installation choice without confusing manual startup with stop.
if [ "$mode" != keep ]; then systemctl start speedbackup-server.service; fi
echo 'WebAdmin: http://127.0.0.1:8765/web/admin (or your configured port)'
systemctl is-enabled speedbackup-server.service || [ "$mode" != on ]
