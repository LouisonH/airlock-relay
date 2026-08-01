#!/bin/sh
set -eu

# Canonical build driver is the Node implementation (target-aware, Tauri
# bundle naming included). This shell wrapper keeps older invocations working.
script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
exec node "$script_dir/build-sidecar.mjs" "$@"
