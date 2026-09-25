#!/bin/sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$ROOT"

SERVER_VERSION=$(sed -n 's/^[[:space:]]*ServerVersion[[:space:]]*=[[:space:]]*"\([^"]*\)".*/\1/p' internal/sbserver/model.go | head -n 1)
[ -n "$SERVER_VERSION" ] || { echo "cannot determine ServerVersion" >&2; exit 1; }
BASE_VERSION=${SERVER_VERSION%%-*}
SUFFIX=${SERVER_VERSION#${BASE_VERSION}}
SUFFIX=${SUFFIX#-}
if [ -n "$SUFFIX" ]; then
  DEB_VERSION="${BASE_VERSION}+${SUFFIX}-1"
else
  DEB_VERSION="${BASE_VERSION}-1"
fi

OUT="$ROOT/dist/packages"
WORK="$ROOT/dist/package-work"
rm -rf "$WORK"
mkdir -p "$OUT" "$WORK"

# Build both Linux release binaries from the exact current source.
if [ "$#" -eq 2 ] && [ "$1" = "--prebuilt-dir" ]; then
  # For packaging cross-compiled binaries in an isolated Linux test host.
  # The caller must retain the source/build hashes with the resulting release.
  for arch in amd64 arm64; do
    test -s "$2/speedbackup-server-$arch"
    install -m 0755 "$2/speedbackup-server-$arch" "$WORK/speedbackup-server-$arch"
  done
elif [ "$#" -eq 0 ]; then
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o "$WORK/speedbackup-server-amd64" ./cmd/speedbackup-server
  CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags='-s -w' -o "$WORK/speedbackup-server-arm64" ./cmd/speedbackup-server
else
  echo "usage: $0 [--prebuilt-dir DIR]" >&2
  exit 2
fi

build_deb() {
  goarch=$1
  debarch=$2
  bin="$WORK/speedbackup-server-$goarch"
  stage="$WORK/deb-$debarch"
  rm -rf "$stage"
  install -d "$stage/DEBIAN" "$stage/usr/bin" "$stage/lib/systemd/system" "$stage/etc/speedbackup-server" "$stage/var/lib/speedbackup-server/data" "$stage/usr/share/doc/speedbackup-server"
  install -m 0755 "$bin" "$stage/usr/bin/speedbackup-server"
  install -m 0644 packaging/linux/systemd/speedbackup-server.service "$stage/lib/systemd/system/speedbackup-server.service"
  install -m 0644 packaging/linux/systemd/speedbackup-server-autostart.socket "$stage/lib/systemd/system/speedbackup-server-autostart.socket"
  install -m 0644 packaging/linux/systemd/speedbackup-server-autostart@.service "$stage/lib/systemd/system/speedbackup-server-autostart@.service"
  install -m 0640 packaging/linux/systemd/server.env "$stage/etc/speedbackup-server/server.env"
  install -m 0644 LICENSE "$stage/usr/share/doc/speedbackup-server/copyright"
  install -m 0644 THIRD_PARTY_NOTICES.txt "$stage/usr/share/doc/speedbackup-server/THIRD_PARTY_NOTICES.txt"
  install -m 0644 README.md "$stage/usr/share/doc/speedbackup-server/README.md"
  install -m 0755 packaging/linux/deb/postinst "$stage/DEBIAN/postinst"
  install -m 0755 packaging/linux/deb/prerm "$stage/DEBIAN/prerm"
  install -m 0755 packaging/linux/deb/postrm "$stage/DEBIAN/postrm"
  printf '/etc/speedbackup-server/server.env\n' > "$stage/DEBIAN/conffiles"
  installed_size=$(du -sk "$stage/usr" "$stage/lib" 2>/dev/null | awk '{n+=$1} END{print n+0}')
  cat > "$stage/DEBIAN/control" <<CONTROL
Package: speedbackup-server
Version: $DEB_VERSION
Section: admin
Priority: optional
Architecture: $debarch
Maintainer: SpeedBackup contributors
Depends: adduser, systemd, util-linux
Installed-Size: $installed_size
Description: SpeedBackup dedicated backup server and WebAdmin
 SpeedBackup Protocol v1 backend with transactional upload sessions,
 manifest generations, integrity verification, and browser WebAdmin.
CONTROL
  pkg="$OUT/speedbackup-server_${DEB_VERSION}_${debarch}.deb"
  dpkg-deb --root-owner-group -Zxz --build "$stage" "$pkg" >/dev/null
  dpkg-deb --info "$pkg" >/dev/null
  dpkg-deb --contents "$pkg" >/dev/null
  ctrl="$WORK/control-$debarch"
  rm -rf "$ctrl" && mkdir -p "$ctrl"
  dpkg-deb --control "$pkg" "$ctrl"
  sh -n "$ctrl/postinst" "$ctrl/prerm" "$ctrl/postrm"
  echo "$pkg"
}

build_deb amd64 amd64
build_deb arm64 arm64

# RPM is optional in the local builder. The checked-in spec is always shipped;
# when rpmbuild is available we also emit real x86_64/aarch64 packages.
if command -v rpmbuild >/dev/null 2>&1; then
  echo "rpmbuild detected; RPM build is enabled"
  for pair in 'amd64 x86_64' 'arm64 aarch64'; do
    set -- $pair
    goarch=$1
    rpmarch=$2
    top="$WORK/rpmbuild-$rpmarch"
    mkdir -p "$top/BUILD" "$top/BUILDROOT" "$top/RPMS" "$top/SOURCES" "$top/SPECS" "$top/SRPMS"
    cp "$WORK/speedbackup-server-$goarch" "$top/SOURCES/speedbackup-server"
    cp packaging/linux/systemd/speedbackup-server.service "$top/SOURCES/"
    cp packaging/linux/systemd/speedbackup-server-autostart.socket "$top/SOURCES/"
    cp packaging/linux/systemd/speedbackup-server-autostart@.service "$top/SOURCES/"
    cp packaging/linux/systemd/server.env "$top/SOURCES/"
    cp LICENSE "$top/SOURCES/LICENSE"
    cp THIRD_PARTY_NOTICES.txt "$top/SOURCES/THIRD_PARTY_NOTICES.txt"
    cp packaging/linux/rpm/speedbackup-server.spec "$top/SPECS/"
    rpmbuild -bb --target "$rpmarch" --define "_topdir $top" "$top/SPECS/speedbackup-server.spec"
    find "$top/RPMS" -type f -name '*.rpm' -exec cp {} "$OUT/" \;
  done
else
  echo "RPM_BUILD_SKIPPED: rpmbuild not installed" > "$OUT/RPM_BUILD_STATUS.txt"
fi

sha256sum "$OUT"/*
