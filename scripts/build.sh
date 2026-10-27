#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
mkdir -p dist
CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 sh scripts/go.sh build -trimpath -buildvcs=false -ldflags='-s -w -buildid=' -o dist/kindle-read-armv7 ./cmd/kindle-read
sh scripts/go.sh build -trimpath -o dist/kindle-read-desktop ./cmd/kindle-read
package_stage=$(mktemp -d /tmp/kindle-read-package.XXXXXX)
trap 'rm -rf "$package_stage"' EXIT HUP INT TERM
sh scripts/go.sh mod vendor -o "$package_stage/vendor"
go_root=$(sh scripts/go.sh env GOROOT)
python3 scripts/package.py "$package_stage" "$go_root"
file dist/kindle-read-armv7
