#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
sh scripts/go.sh test -race ./...
sh scripts/go.sh vet ./...
npm test
git diff --check
