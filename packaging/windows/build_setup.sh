#!/bin/sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$ROOT"
VERSION=$(sed -n 's/^[[:space:]]*ServerVersion[[:space:]]*=[[:space:]]*"\([^"]*\)".*/\1/p' internal/sbserver/model.go | head -n 1)
[ -n "$VERSION" ] || { echo "cannot determine ServerVersion" >&2; exit 1; }
OUT="$ROOT/dist/windows-amd64"
mkdir -p "$OUT"
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o "$OUT/speedbackup-server.exe" ./cmd/speedbackup-server
cp packaging/windows/install_service_admin.ps1 "$OUT/"
cp README.md LICENSE start_windows.ps1 "$OUT/"
cat > "$OUT/BUILD_ON_WINDOWS.txt" <<EOT
This release no longer ships the custom Go setup EXE.
From the FULL SOURCE root, build with Inno Setup 6.7 or newer:

  powershell -ExecutionPolicy Bypass -File .\\packaging\\windows\\compile_prebuilt_installer.ps1

Or rebuild server.exe and setup together:

  powershell -ExecutionPolicy Bypass -File .\\packaging\\windows\\build_installer_windows.ps1

The source installer script is:
  packaging\\windows\\speedbackup-server.iss

Expected output:
  dist\\windows-setup\\SpeedBackup_Server_Setup_v${VERSION}_windows_x64.exe
EOT
file "$OUT/speedbackup-server.exe"
sha256sum "$OUT/speedbackup-server.exe"
