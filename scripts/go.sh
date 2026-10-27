#!/bin/sh
set -eu
project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
go_compiler=${KINDLE_READ_GO:-"$project_dir/.tools/go/bin/go"}
if [ ! -x "$go_compiler" ]; then
    echo 'Go toolchain missing. Run: sh scripts/setup.sh' >&2
    exit 1
fi
export GOCACHE=${GOCACHE:-/tmp/kindle-read-go-cache}
export GOPATH=${GOPATH:-/tmp/kindle-read-gopath}
exec "$go_compiler" "$@"
