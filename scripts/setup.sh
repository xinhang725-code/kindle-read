#!/bin/sh
set -eu
project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_dir"
case "$(uname -s)-$(uname -m)" in Linux-x86_64) ;; *) echo 'Pinned bootstrap supports Linux x86_64; set KINDLE_READ_GO for other development hosts.' >&2; exit 1 ;; esac
archive_name=go1.25.3.linux-amd64.tar.gz
archive_sha=0335f314b6e7bfe08c3d0cfaa7c19db961b7b99fb20be62b0a826c992ad14e0f
mkdir -p .tools
if [ ! -x .tools/go/bin/go ]; then
    curl --fail --location --proto '=https' --tlsv1.2 "https://go.dev/dl/$archive_name" -o ".tools/$archive_name"
    printf '%s  %s\n' "$archive_sha" ".tools/$archive_name" | sha256sum --check -
    tar -xzf ".tools/$archive_name" -C .tools
    rm ".tools/$archive_name"
fi
sh scripts/go.sh version
sh scripts/go.sh mod download
npm ci --cache /tmp/kindle-read-npm-cache
echo 'Setup complete. Run: sh scripts/check.sh'
