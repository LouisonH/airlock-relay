#!/bin/sh
set -eu

release_script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
release_repo_root=$(CDPATH= cd -- "$release_script_dir/.." && pwd)
release_version=$(node -p "require('$release_repo_root/apps/desktop/package.json').version")
release_target=${1:-${AIRLOCK_TARGET:-}}
release_target_explicit=false

if [ -n "$release_target" ]; then
  release_target_explicit=true
else
  case "$(uname -s):$(uname -m)" in
    Darwin:arm64) release_target=darwin-arm64 ;;
    Darwin:x86_64) release_target=darwin-amd64 ;;
    Linux:x86_64) release_target=linux-amd64 ;;
    Linux:aarch64) release_target=linux-arm64 ;;
    Linux:armv7l) release_target=linux-armv7 ;;
    *)
      echo "unsupported host; set AIRLOCK_TARGET to a supported target" >&2
      exit 2
      ;;
  esac
fi

release_goos=
release_goarch=
release_goarm=
release_extension=
case "$release_target" in
  darwin-arm64) release_goos=darwin; release_goarch=arm64 ;;
  darwin-amd64) release_goos=darwin; release_goarch=amd64 ;;
  windows-amd64) release_goos=windows; release_goarch=amd64; release_extension=.exe ;;
  windows-arm64) release_goos=windows; release_goarch=arm64; release_extension=.exe ;;
  linux-amd64) release_goos=linux; release_goarch=amd64 ;;
  linux-arm64) release_goos=linux; release_goarch=arm64 ;;
  linux-armv7) release_goos=linux; release_goarch=arm; release_goarm=7 ;;
  *)
    echo "unsupported Airlock target: $release_target" >&2
    exit 2
    ;;
esac

if [ "$(uname -s)" = "Darwin" ]; then
  export MACOSX_DEPLOYMENT_TARGET="${MACOSX_DEPLOYMENT_TARGET:-12.0}"
  release_go_cache="${AIRLOCK_GO_CACHE:-$release_repo_root/.cache/go-build-macos-$MACOSX_DEPLOYMENT_TARGET}"
  mkdir -p "$release_go_cache"
  export GOCACHE="$release_go_cache"
fi

if [ "$release_target_explicit" = true ]; then
  release_output_dir=${AIRLOCK_OUTPUT_DIR:-$release_repo_root/bin/$release_target}
else
  # Keep the desktop development sidecar location stable for Tauri.
  release_output_dir=${AIRLOCK_OUTPUT_DIR:-$release_repo_root/bin}
fi

mkdir -p "$release_output_dir"
cd "$release_repo_root"

if [ "$release_target_explicit" = true ]; then
  export GOOS="$release_goos"
  export GOARCH="$release_goarch"
  if [ -n "$release_goarm" ]; then
    export GOARM="$release_goarm"
  fi
  if [ "$release_goos" != darwin ]; then
    export CGO_ENABLED=0
  fi
fi

go build -buildvcs=false -trimpath -ldflags "-s -w -X main.version=$release_version" -o "$release_output_dir/airlockd$release_extension" ./cmd/airlockd
go build -buildvcs=false -trimpath -ldflags "-s -w -X main.version=$release_version" -o "$release_output_dir/airlock$release_extension" ./cmd/airlock
