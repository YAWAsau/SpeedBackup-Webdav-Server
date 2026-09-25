#!/bin/sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
UNIT="$ROOT/packaging/linux/systemd/speedbackup-server.service"
ENVF="$ROOT/packaging/linux/systemd/server.env"
POSTINST="$ROOT/packaging/linux/deb/postinst"
PRERM="$ROOT/packaging/linux/deb/prerm"
POSTRM="$ROOT/packaging/linux/deb/postrm"
SPEC="$ROOT/packaging/linux/rpm/speedbackup-server.spec"
WINISS="$ROOT/packaging/windows/modern.iss"
WINREADME="$ROOT/packaging/windows/README_WINDOWS_INSTALLER.md"
WINBUILD="$ROOT/packaging/windows/build_installer_windows.ps1"
WINPREBUILT="$ROOT/packaging/windows/compile_prebuilt_installer.ps1"
for f in "$UNIT" "$ENVF" "$POSTINST" "$PRERM" "$POSTRM" "$SPEC" "$WINISS" "$WINREADME" "$WINBUILD" "$WINPREBUILT"; do
  [ -f "$f" ] || { echo "missing: $f" >&2; exit 1; }
done
grep -Fq 'User=speedbackup' "$UNIT"
grep -Fq 'Group=speedbackup' "$UNIT"
grep -Fq 'EnvironmentFile=-/etc/speedbackup-server/server.env' "$UNIT"
grep -Fq 'ExecStart=/usr/bin/speedbackup-server serve --root ${SPEEDBACKUP_ROOT} --listen ${SPEEDBACKUP_LISTEN}' "$UNIT"
grep -Fq 'SPEEDBACKUP_ROOT=/var/lib/speedbackup-server/data' "$ENVF"
grep -Fq 'SPEEDBACKUP_LISTEN=0.0.0.0:8765' "$ENVF"
sh -n "$POSTINST" "$PRERM" "$POSTRM"
# Removal must preserve backup/state by default.
if grep -E 'rm[[:space:]]+-rf[[:space:]]+.*var/lib/speedbackup-server' "$PRERM" "$POSTRM" >/dev/null 2>&1; then
  echo 'destructive /var/lib removal found' >&2
  exit 1
fi
grep -Fq '%systemd_post speedbackup-server.service' "$SPEC"
grep -Fq '%systemd_preun speedbackup-server.service' "$SPEC"
grep -Fq '%systemd_postun_with_restart speedbackup-server.service' "$SPEC"
grep -Fq '[Setup]' "$WINISS"
grep -Fq '[Run]' "$WINISS"
grep -Fq 'service install --root' "$WINISS"
grep -Fq 'service start' "$WINISS"
grep -Fq 'FIRST_RUN_TOKEN.txt' "$WINISS"

# Windows Inno builder must resolve ISCC.exe to a plain string path.
# A FileInfo returned by Get-Item has FullName but no Source; invoking $iscc.Source
# becomes '& <null>' on Windows and fails before ISCC starts.
for ps in "$WINBUILD" "$WINPREBUILT" "$ROOT/build_windows.ps1"; do
  if grep -F '\$iscc.Source' "$ps" >/dev/null 2>&1; then
    echo "PowerShell Inno invocation uses invalid \$iscc.Source: $ps" >&2
    exit 1
  fi
done
grep -Fq 'Resolve-IsccPath' "$ROOT/packaging/windows/build_common.ps1"
grep -Fq 'modern dynamic windows11' "$WINISS"
grep -Fq 'Type=notify' "$UNIT"
grep -Fq 'cmd/speedbackup-server-setup' "$ROOT/build_windows.ps1" && { echo 'retired custom Go setup target still used by build_windows.ps1' >&2; exit 1; } || true
grep -Fq 'cmd/speedbackup-server-setup' "$ROOT/packaging/windows/build_setup.sh" && { echo 'retired custom Go setup target still used by build_setup.sh' >&2; exit 1; } || true

[ ! -d "$ROOT/cmd/speedbackup-server-setup" ] || { echo 'retired custom Go setup command still exists' >&2; exit 1; }
[ ! -d "$ROOT/internal/installer" ] || { echo 'retired custom Go installer package still exists' >&2; exit 1; }
echo 'packaging static verification: OK'
