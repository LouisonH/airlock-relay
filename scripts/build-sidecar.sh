#!/bin/sh
set -eu

release_script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
release_repo_root=$(CDPATH= cd -- "$release_script_dir/.." && pwd)
release_version=$(node -p "require('$release_repo_root/apps/desktop/package.json').version")

if [ "$(uname -s)" = "Darwin" ]; then
  export MACOSX_DEPLOYMENT_TARGET="${MACOSX_DEPLOYMENT_TARGET:-12.0}"
  release_go_cache="${AIRLOCK_GO_CACHE:-$release_repo_root/.cache/go-build-macos-$MACOSX_DEPLOYMENT_TARGET}"
  mkdir -p "$release_go_cache"
  export GOCACHE="$release_go_cache"
fi

mkdir -p "$release_repo_root/bin"
cd "$release_repo_root"
go build -buildvcs=false -trimpath -ldflags "-s -w -X main.version=$release_version" -o bin/airlockd ./cmd/airlockd
