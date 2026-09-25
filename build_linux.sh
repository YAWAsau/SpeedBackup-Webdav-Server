#!/usr/bin/env sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$ROOT"
mkdir -p dist/linux-amd64 dist/linux-arm64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o dist/linux-amd64/speedbackup-server ./cmd/speedbackup-server
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags='-s -w' -o dist/linux-arm64/speedbackup-server ./cmd/speedbackup-server
sha256sum dist/linux-amd64/speedbackup-server dist/linux-arm64/speedbackup-server
